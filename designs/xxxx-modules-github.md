
## GitHub Support

A large percentage of CUE users have their schema in Github repositories. We
will provide a GitHub app that will simplify publishing modules to the registry.
In the first implementation phase, this GitHub app will also handle
authorization.

### Basic design

- A GitHub user can install the Github app to their personal account, or an
  organization they control, to automatically publish certain repos to the CUE
  registry.
- Within a repository, a new module version is published by tagging a commit
  with `cue-v$semver`, where the `$semver` part must be a valid SemVer tag. e.g.
  `cue-v1.2.3`.
- A repository may have multiple CUE modules, each indicated by a `cue.mod`
  subdirectory. Although these may not be nested.
- The module path of each module must be of the form
  `github.com/$owner/$repo/path@$majorversion` (where `$owner` could be a user
  or organization) and correspond to the location in the repository.
- For a given repository, only tags with the same major version are published.
- Otherwise, a module is only published if:
    - the `cue.mod/module.cue` file is well-formed
    - all module dependencies exist
    - all backwards compatibility guarantees are met

### Rationale

GitHub support is implemented as a separate layer rather than integrated
directly into the registry. This:

- Keeps the core registry simple.
- Allows us to evolve the complexities of dealing with VCS support without
  changing the registry core, which has a simple interface.
- Allows for various GitHub apps for different use cases.
- We can piggyback on the GitHub authentication for a first implementation of
  the registry. The ability of a user to install the app in a user or
  organization also provides authorization that they control that namespace.

Proposed Github App design:

- The main aim of the design is to have something availabe quickly.
- We anticipate eventually replacing the app with a new solution that offers
  better feedback to the user regarding any issues that may arise during use.
