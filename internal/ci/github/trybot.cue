// Copyright 2022 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import (
	"list"
	"cue.dev/x/githubactions"
)

// The trybot workflow.
workflows: trybot: _repo.bashWorkflow & {
	name: _repo.trybot.name

	on: {
		push: {
			branches: list.Concat([[_repo.testDefaultBranch], _repo.protectedBranchPatterns]) // do not run PR branches
		}
		pull_request: {}
	}

	jobs: test: {
		"runs-on": _repo.linuxMachine

		// Only run the trybot workflow if we have the trybot trailer, or
		// if we have no special trailers. Note this condition applies
		// after and in addition to the "on" condition above.
		if: "\(_repo.containsTrybotTrailer) || ! \(_repo.containsDispatchTrailer)"

		steps: [
			for v in _repo.checkoutCode {v},
			for v in _repo.installGo {v},
			for v in _repo.setupCaches {v},

			// CUE setup
			_installCUE,

			_repo.earlyChecks,
			_centralRegistryLogin,
			_#goGenerate,
			_#goTest,
			_#goCheck,
			_repo.checkGitClean,
		]
	}

	_#goGenerate: githubactions.#Step & {
		name: "Generate"
		run:  "go generate ./..."
	}

	_#goTest: githubactions.#Step & {
		name: "Test"
		run:  "go test ./..."
	}

	_#goCheck: githubactions.#Step & {
		// These checks can vary between platforms, as different code can be built
		// based on GOOS and GOARCH build tags.
		// However, CUE does not have any such build tags yet, and we don't use
		// dependencies that vary wildly between platforms.
		// For now, to save CI resources, just run the checks on one matrix job.
		// TODO: consider adding more checks as per https://github.com/golang/go/issues/42119.
		name: "Check"
		run:  "go vet ./..."
	}
}

_installCUE: githubactions.#Step & {
	name: "Install CUE"
	uses: "cue-lang/setup-cue@v1.0.1"
	with: version: "latest"
}

_centralRegistryLogin: githubactions.#Step & {
	env: {
		// Note: this token has read-only access to the registry
		// and is used only because we need some credentials
		// to pull dependencies from the Central Registry.
		// The token is owned by notcueckoo and described as "ci readonly".
		CUE_TOKEN: "${{ secrets.NOTCUECKOO_CUE_TOKEN }}"
	}
	run: """
		cue login --token=${CUE_TOKEN}
		"""
}
