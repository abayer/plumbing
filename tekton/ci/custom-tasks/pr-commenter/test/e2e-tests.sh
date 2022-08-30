\#!/usr/bin/env bash

# Copyright 2020 The Tekton Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script calls out to scripts in tektoncd/plumbing to setup a cluster
# and deploy Tekton Pipelines to it for running integration tests.

source $(git rev-parse --show-toplevel)/tekton/ci/custom-tasks/test/e2e-common.sh
source $(git rev-parse --show-toplevel)/tekton/ci/custom-tasks/pr-commenter/test/e2e-pr-commenter.sh

# initialize function does a CD to REPO_ROOT_DIR so we have to CD back here.
cd ${REPO_ROOT_DIR}/tekton/ci/custom-tasks/pr-commenter

header "Setting up environment"

install_pipeline_crd_version latest

print "Setting feature gate to alpha\n"
jsonpatch=$(print "{\"data\": {\"enable-api-fields\": \"alpha\"}}")
echo "feature-flags ConfigMap patch: ${jsonpatch}"
kubectl patch configmap feature-flags -n tekton-pipelines -p "$jsonpatch"

install_pr_commenter_crd

failed=0

# Run the integration tests
header "Running Go e2e tests"
go_test_e2e -timeout=20m ./test/... || failed=1

(( failed )) && fail_test
success
