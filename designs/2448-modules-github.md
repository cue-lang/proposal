# Proposal: Module-publishing Github app
Status: **Draft**

Lifecycle:  **Proposed**

Author(s): rog@cue.works

Relevant Links: [https://www.notion.so/Module-sub-proposals-fff51d3e1bdd4f98aa24a3a670c97e5e](https://www.notion.so/Module-sub-proposals-fff51d3e1bdd4f98aa24a3a670c97e5e?pvs=21)

Reviewers: mpvl@cue.works myitcv@cue.works

Approvers: name2@

Discussion Channel [Discord, GitHub, Slack, Project Management tool?]: {link}

## Objective

A large percentage of CUE users have their schema in Github repositories. We
will provide a GitHub app that will simplify publishing modules to the registry.
In the first implementation phase, this GitHub app will also handle
authorization.

### Design

Initial support for modules will be limited to public repositories inside Github. All published modules will be of the form `github.com/user/repo`. To publish a module to the registry, a user will install the CUE Module Publisher app and push a tag that tags a version of the module.

The app will ask for exactly as many permissions are strictly necessary to perform its function:

- Metadata: Read-Only (this is required by Github)
- Contents: Read-Only

The app will see the tag being pushed, verify that the module looks OK and, if so, publish it to the central repository.

The tag must be of the form `cue-v$semver`, for example, `cue-v0.2.3`.

Generally, if the `cue mod tidy` operation runs without any errors and doesn't make any changes, the content should be accepted into the store. This is subject to exceptions such as concurrent tags to the same repository or tag mutation.

### Verification

Before a module is made available in the central repository, some verification steps are taken
to make sure that it looks valid.

Specifically, the app will check (at least) that, for some github-hosted module M:

- M has a valid `module.cue` file that declares the correct github repository
- All dependencies within M represent the final MVS results for module resolution.
- All dependencies within M are correctly resolved to the officially recognized content for those dependencies
- The CUE code within M is syntactically valid.

In time, the app will also add signed statements to attest to the above.
See discussion in the [supply-chain security sub-proposal](TODO).

### Rationale

The decision to require a `cue-` prefix to tags came from experiences with the Go ecosystem where conflicts with existing pre-Go tags caused many problems. By requiring a CUE-specific tag name, we hope to sidestep conflicts with existing tags, albeit at the cost of adding another tag.

GitHub support is implemented as a separate layer rather than integrated
directly into the registry. This:

- Keeps the core registry simple, and allows us to avoid using an entirely custom implementation.
- Allows us to evolve the complexities of dealing with VCS support without
  changing the registry core, which has a simple interface.
- Allows for various GitHub apps for different use cases.
- We can piggyback on the GitHub authentication for a first implementation of the registry. The ability of a user to install the app in a user or organization also provides authorization that they control that namespace.

### Issues

At least as currently conceived, there's a lack of feedback from the app to the user regarding failed uploads. There is no obvious way for the app to provide feedback because there is no PR or issue to post comments from the app to. We hope that having cue mod tidy do as many checks as possible will mitigate that problem. In the future we will have some way for a user to see the results of the central repository checks.
