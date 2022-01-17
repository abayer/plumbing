package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tektoncd/plumbing/bots/tep-automation/pkg/ghclient"
	"github.com/tektoncd/plumbing/bots/tep-automation/pkg/tep"
)

func main() {
	if err := NewFromRepoCmd(context.Background()).Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// FromRepoOptions contains the CLI flags for `tep-issues-from-repo`
type FromRepoOptions struct {
	GitHubToken string
}

// NewFromRepoCmd returns the tep-issues-from-repo command
func NewFromRepoCmd(ctx context.Context) *cobra.Command {
	options := &FromRepoOptions{}

	cobraCmd := &cobra.Command{
		Use:   "tep-issues-from-repo",
		Short: "Create tracking issues for existing TEPs",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.run(ctx); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
		DisableAutoGenTag: true,
	}

	cobraCmd.Flags().StringVar(&options.GitHubToken, "github-token", "", "GitHub token to use for issue creation")
	_ = cobraCmd.MarkFlagRequired("github-token")

	return cobraCmd
}

func (o *FromRepoOptions) run(ctx context.Context) error {
	return o.CreateIssues(ctx, ghclient.NewTEPGHClientFromToken(ctx, o.GitHubToken))
}

// CreateIssues creates tracking issues for all open TEPs in the README which don't already have issues.
func (o *FromRepoOptions) CreateIssues(ctx context.Context, tgc *ghclient.TEPGHClient) error {
	// Get all TEPs from the README
	allTEPs, err := tgc.GetTEPsFromReadme(ctx)
	if err != nil {
		return err
	}

	// Filter the TEPs for only those not in a terminal status
	activeTEPs := make(map[string]tep.TEPInfo)
	for tepID, tepInfo := range allTEPs {
		if !tepInfo.Status.IsTerminalStatus() {
			activeTEPs[tepID] = tepInfo
		}
	}

	// Get all existing tracking issues
	trackingIssues, err := tgc.GetTrackingIssues(ctx, &ghclient.GetTrackingIssuesOptions{})
	if err != nil {
		return err
	}

	// Iterate over the active TEPs, and create issues for any which don't already have tracking issues. The issue won't
	// have any PR references in it, since we can't get that information from the TEPInfo.
	for tepID, tepInfo := range activeTEPs {
		if _, ok := trackingIssues[tepID]; !ok {
			log.Printf("Creating tracking issue for TEP-%s\n", tepID)

			parsedInfo, err := tgc.TEPInfoFromRepo(ctx, tepInfo.ID, tepInfo.Filename)
			if err != nil {
				return errors.Wrapf(err, "loading TEP-%s info from Markdown file %s in repository", tepInfo.ID, tepInfo.Filename)
			}

			issue := tep.TrackingIssue{
				TEPStatus: parsedInfo.Status,
				TEPID:     parsedInfo.ID,
				Assignees: parsedInfo.Authors,
			}
			issueBody, err := issue.GetBody(parsedInfo.Filename)
			if err != nil {
				return errors.Wrapf(err, "couldn't generate issue body for TEP-%s", tepInfo.ID)
			}
			if err := tgc.CreateTrackingIssue(ctx, issue.TEPID, issueBody, issue.Assignees, issue.TEPStatus); err != nil {
				return errors.Wrapf(err, "creating tracking issue for TEP-%s", tepInfo.ID)
			}
		}
	}

	return nil
}
