package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v41/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	main "github.com/tektoncd/plumbing/bots/tep-automation/cmd/util"
	"github.com/tektoncd/plumbing/bots/tep-automation/pkg/ghclient"
	"github.com/tektoncd/plumbing/bots/tep-automation/pkg/tep"
	"github.com/tektoncd/plumbing/bots/tep-automation/pkg/testutil"
)

const (
	readmeContent = `there are five teps in here
on later lines
|[TEP-1234](1234-no-issue.md) | No Issue TEP | proposed | 2021-12-20 |
|[TEP-2341](2341-has-issue.md) | Existing Issue TEP | proposed | 2021-12-20 |
|[TEP-5678](5678-implemented.md) | Implemented TEP | implemented | 2020-05-14 |
|[TEP-6785](6785-withdrawn.md) | Withdrawn TEP | withdrawn | 2020-05-14 |
|[TEP-7856](7856-replaced.md) | Replaced TEP | replaced | 2020-05-14 |
tada, five valid TEPs
`
)

func TestFromRepo(t *testing.T) {
	ctx := context.Background()

	ghClient, mux, closeFunc := testutil.SetupFakeGitHub()
	defer closeFunc()

	tgc := ghclient.NewTEPGHClient(ghClient)

	mux.HandleFunc(testutil.ReadmeURL, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, testutil.GHContentJSON(readmeContent))
	})

	mdFiles, err := ioutil.ReadDir("testdata")
	require.NoError(t, err)
	for _, f := range mdFiles {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			contentBytes, err := ioutil.ReadFile(filepath.Join("testdata", f.Name()))
			contentStr := string(contentBytes)
			require.NoError(t, err)
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/contents/%s/%s", ghclient.TEPsOwner, ghclient.TEPsRepo,
				ghclient.TEPsDirectory, f.Name()),
				func(w http.ResponseWriter, r *http.Request) {
					_, _ = fmt.Fprint(w, testutil.GHContentJSON(contentStr))
				})
		}
	}

	existingIssues := []*github.Issue{{
		Title:  github.String("TEP-2341 Tracking Issue"),
		Number: github.Int(1),
		State:  github.String("open"),
		Assignees: []*github.User{
			{
				Login: github.String("alice"),
			},
			{
				Login: github.String("charles"),
			},
		},
		Labels: []*github.Label{
			{
				Name: github.String(ghclient.TrackingIssueLabel),
			},
			{
				Name: github.String(tep.ProposedStatus.TrackingLabel()),
			},
		},
		Body: github.String(`<!-- TEP PR: 55 -->
<!-- Implementation PR: repo: pipeline number: 77 -->
<!-- Implementation PR: repo: triggers number: 88 -->`),
	}}

	createdIssues := []testutil.ExpectedIssue{{
		TrackingIssue: tep.TrackingIssue{
			TEPStatus: tep.ProposedStatus,
			TEPID:     "1234",
			Assignees: []string{"someone", "bob"},
		},
		Filename: "1234-no-issue.md",
	}}

	createCalls := 0

	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/issues", ghclient.TEPsOwner, ghclient.TEPsRepo),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				respBody, err := json.Marshal(existingIssues)
				if err != nil {
					t.Fatal("marshalling GitHub issue list")
				}
				_, _ = fmt.Fprint(w, string(respBody))
			} else if r.Method == "POST" {
				v := new(github.IssueRequest)
				require.NoError(t, json.NewDecoder(r.Body).Decode(v))

				matchedIR := false

				for _, created := range createdIssues {
					ir := created.ToIssueRequest(t)

					if cmp.Equal(ir, v) {
						matchedIR = true
					}
				}

				if !matchedIR {
					unknownReq, _ := json.MarshalIndent(v, "", "  ")
					t.Fatalf("received unexpected IssueRequest:\n%s", string(unknownReq))
				}

				createCalls++
				_, _ = fmt.Fprint(w, `{"number":1}`)
			}
		})

	cmd := &main.FromRepoOptions{}

	require.NoError(t, cmd.CreateIssues(ctx, tgc))
	assert.Equalf(t, len(createdIssues), createCalls, "expected %d issues to be created, but got %d", len(createdIssues), createCalls)
}
