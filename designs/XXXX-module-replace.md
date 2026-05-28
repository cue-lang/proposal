# Proposal: Module Replace Directives with a Two-File Approach

Author: Roger Peppe
Date: 2026-05-27
Status: Draft

## Abstract

We propose adding `replace` directive support to the CUE module
system, covering both directory replacements (for local development)
and module-version replacements (for fork-based workflows). Rather
than placing replace directives in the existing `cue.mod/module.cue`
file, we introduce an optional companion file `cue.mod/local-module.cue`
that holds replace directives and an alternative dependency list.
This two-file design cleanly separates published module metadata
from local development configuration, avoiding the problems that Go
encountered with replace directives in the single `go.mod` file.

## Background

CUE's module system, broadly modeled on Go modules, currently
supports declaring dependencies with version constraints in
`cue.mod/module.cue`. The `deps` map holds module paths to `#Dep`
entries containing a version (`v`) and an optional `default` flag.
The existing schema already anticipates replacement support: the
`#Dep` type allows `v!: #Semver | null` with a comment noting
that null is permitted "if the version is unknown and the module
entry is only present to be replaced", and `#Strict` explicitly
notes "No null versions, because no replacements yet."

No replace or exclude functionality has been implemented.

### The need for replace directives

The most pressing use case is the "edit two modules at once"
workflow. A developer working on module A that depends on module B
needs to make coordinated changes to both. Without replace
directives, changes to B must be published (or at least pushed to a
registry) before they can be tested in A, creating a painful
publish-test-fix cycle.

Directory replace directives solve this directly: point module A's
dependency on B at a local checkout, make changes to both, test them
together, and publish only when everything works.

Module-version replace directives address a secondary but
well-understood use case: redirecting a dependency to a fork or
alternative module at a specific version, typically when a dependency
has a bug and the developer needs a patched fork.

### Go's approach and its limitations

Go modules provide `replace` and `exclude` directives in `go.mod`.
These are honored only in the main module; they are ignored when the
module is used as a dependency. This ensures that replace and exclude
are tools for the developer's own workflow, not constraints imposed
on downstream consumers.

However, placing replace directives in the single `go.mod` file has
a structural limitation: only one set of dependencies can be
represented. In Go, `go.mod` holds the main-module dependencies
(i.e., the dependency graph as resolved with replaces applied). When
directory replaces point at local checkouts, the resolved versions
may differ from what would be resolved without those replaces. This
means `go.mod` does not faithfully represent the dependency graph
that downstream consumers of the module will see.

Go addressed the "accidental commit of directory replaces" problem
by introducing `go.work` workspace files in Go 1.18. Workspace
files live outside the module directory and are typically
`.gitignore`d. While effective, this was a post-hoc addition that
required new tooling and introduced a separate configuration surface.

### CUE's opportunity

CUE can learn from Go's experience and design the two-file split
from the start. By introducing `cue.mod/local-module.cue` as the
designated home for replace directives and main-module-specific
dependency overrides, we avoid the need for a separate workspace
mechanism while maintaining a clean separation between published
and development-time metadata.

## Proposal

### Two-file structure

The module system gains an optional second file:
`cue.mod/local-module.cue`. The two files have distinct roles:

**`cue.mod/module.cue`** (existing) contains:
- The module path (`module`).
- The language version (`language: version`).
- Source metadata (`source`).
- Dependencies (`deps`) as they would be resolved without any
  replace directives, representing the module as seen by downstream
  consumers.
- Custom tool data (`custom`).

**`cue.mod/local-module.cue`** (new, optional) contains:
- Dependencies (`deps`) as resolved with replaces applied,
  representing the main-module view. Individual dep entries may
  include a `replace` field specifying the replacement target.
- The `default` annotations that correspond to the main-module
  dependency set.

Only `module.cue` contains the module path and language version.
Only dep entries in `local-module.cue` may contain `replace` fields.

When `local-module.cue` is absent, the module system behaves exactly
as it does today: `module.cue` is the sole source of truth.

When `local-module.cue` is present, the CUE tooling merges
information from both files. For dependency resolution in main-module
mode, the dependencies in `local-module.cue` take precedence, with
replace directives applied. For publishing or when the module is
used as a dependency, only `module.cue` is consulted.

### Lax file syntax and bootstrapping

Both files share the same "lax" syntax (the existing `#File` schema
minus `#Strict` constraints). This means a developer can bootstrap
`local-module.cue` by copying `module.cue`:

```
cp cue.mod/module.cue cue.mod/local-module.cue
```

The resulting file is valid; `cue mod tidy` then normalizes it by
removing the `module` and `language: version` fields from
`local-module.cue` (since those belong exclusively in `module.cue`).
If `cue mod tidy` finds an incompatibility between the two files, it
reports an error: a mismatched module path is a hard error; differing
language versions resolve to the greater of the two (with a warning).

### Replace field syntax

Replacements are expressed as a `replace` field within individual
`deps` entries in `local-module.cue`. Two forms are supported:

**Directory replacement** redirects a module to a local filesystem
path:

```cue
// local-module.cue
deps: "example.com/foo@v0": {
    v: "v0.0.1"
    replace: "../local-foo"
}
```

Both relative and absolute paths are supported. Relative paths are
resolved relative to the directory containing `cue.mod`. On Windows,
absolute paths with drive letters are recognized:

```cue
deps: "example.com/foo@v0": {
    v: "v0.0.1"
    replace: "C:/Users/dev/local-foo"
}
```

**Module-version replacement** redirects a module to a different
module and version in the registry:

```cue
// local-module.cue
deps: "example.com/foo@v0": {
    v: "v0.0.1"
    replace: "example.com/bar@v0.1.2"
}
```

The replacement module is fetched from the registry in the usual
way. The replacement target is a full module path with version,
indicating the specific version to use.

The `replace` value is interpreted as a directory path if it starts
with `.` or `/`, or if it matches a Windows absolute path (a drive
letter followed by a colon, e.g., `C:\libs\foo` or `C:/libs/foo`).
Otherwise it is interpreted as a module path with version. This
disambiguation is unambiguous because valid module paths cannot
start with `.` or `/` and cannot contain a colon (which appears in
the second character of a Windows drive-letter path).

### Schema changes

The `#Dep` type gains an optional `replace` field:

```cue
#Dep: {
    v!: #Semver | null
    default?: bool

    // replace specifies a replacement for this dependency.
    // A value starting with "." or "/", or matching a Windows
    // absolute path (e.g. "C:\..." or "C:/..."), is a directory
    // path; otherwise it is a module path with version.
    replace?: string
}
```

The `#Strict` schema (used at publish time) rejects the `replace`
field:

```cue
#Strict: #File & {
    // ... existing strict constraints ...

    // Replacements are not permitted in published modules.
    #Dep: replace?: _errorReplaceNotPermittedInStrict
}
```

This co-locates the replacement with the dependency it affects,
making the relationship explicit and avoiding a separate top-level
map that must be kept in sync with `deps`.

### Dependency synchronization with `cue mod tidy`

`cue mod tidy` keeps dependencies synchronized between the two
files:

1. It resolves the full dependency graph twice: once with replace
   directives applied (the main-module view) and once without (the
   published-module view).

2. The main-module dependencies are written to `local-module.cue`;
   the published-module dependencies are written to `module.cue`.

3. For any module present in both files, the version is set to the
   maximum of the two resolved versions. This ensures that the
   published `module.cue` is never behind the main-module view.

4. The `default: true` annotations are kept in sync between the two
   files.

5. If no dep entry in `local-module.cue` contains a `replace` field
   after tidying, `cue mod tidy` removes the file entirely, since
   it serves no purpose without replacements.

#### The `--local-only` flag

`cue mod tidy --local-only` restricts the tidy operation to
`local-module.cue` only, leaving `module.cue` unchanged. This is
useful when the replacement targets do not exist in any registry
(e.g., a module that exists only as a local checkout) and the
published-module dependency resolution would fail.

### Replacement upgrades and MVS

A situation unique to CUE (and not possible in Go) arises when a
replacement target is itself a dependency at a different version.

Consider:
- The main module depends on `bar.com@v0.0.1` and `baz.com@v0.0.1`.
- `bar.com@v0.0.1` is replaced by `foo.com@v0.1.2`.
- `baz.com@v0.0.1` depends on `foo.com@v0.2.0`.

In Go, this cannot occur because `foo.com` must declare itself as
`bar.com` for the replacement to work. In CUE, however, replacements
do not require the target to declare a matching module path, so this
situation is possible.

The resolution is to apply MVS as usual: `foo.com` appears in the
dependency graph both as a replacement target at `v0.1.2` and as a
transitive dependency at `v0.2.0`. MVS selects the maximum,
`v0.2.0`, and the replace directive is updated accordingly. The
developer sees the upgrade reflected in `local-module.cue` after
running `cue mod tidy`.

### Interaction with registries

When publishing a module to a registry, only `module.cue` is
included. The `local-module.cue` file is omitted from the module zip
because it contains development-time configuration that is
irrelevant to consumers. The registry's `#Strict` validation
rejects any `module.cue` that contains replace directives, providing
a safety net.

This is the key advantage of the two-file design: there is no risk
of accidentally publishing replace directives, because they live in
a file that is excluded from publication by construction.

### Interaction with `cue.work` (future)

A future workspace mechanism (analogous to Go's `go.work`) could
complement `local-module.cue` by providing cross-module workspace
configuration that lives entirely outside any individual module. The
two features are orthogonal: `local-module.cue` handles per-module
replace directives, while a workspace file would handle multi-module
development layouts. The design of `local-module.cue` does not
preclude or constrain a future workspace feature.

## Rationale

### Why not put replace directives in `module.cue`?

Placing replace directives in `module.cue` would replicate the
problem Go had before `go.work`: developers must remember to remove
directory replaces before committing, and the single dependency list
cannot represent both the main-module and published-module views.
The two-file split avoids both problems.

### Why not a workspace file?

A workspace file (like `go.work`) solves the "don't commit directory
replaces" problem but introduces a separate configuration surface
that lives outside the module. For the common case of a single
module with a few replace directives, a companion file inside
`cue.mod/` is simpler and more discoverable. A workspace mechanism
may still be valuable for multi-module monorepo workflows, but it is
a larger design that need not block replace directive support.

### Why not `.gitignore` `local-module.cue` by default?

While directory replaces are inherently non-portable and poor
candidates for version control, module-version replaces can be
useful to share across a team (e.g., pointing to a patched fork
until a fix is upstreamed). Making the file always gitignored would
prevent this. Instead, we leave the version control decision to the
developer. Tooling can warn when `local-module.cue` contains
directory replaces and is tracked by git.

### Why merge dependency versions to the maximum?

Taking the maximum version across both files ensures that
`module.cue` never advertises a dependency version lower than what
the main module actually requires. This prevents a scenario where a
consumer fetches `module.cue`, resolves dependencies, and ends up
with a version that the module's code has never been tested against.

## Compatibility

This proposal is fully backward compatible. Existing modules that
have no `local-module.cue` file behave identically to the current
system. The schema changes are additive: the `replace` field on
`#Dep` is optional and only permitted in `local-module.cue`.

The proposal requires a new language version gate (e.g.,
`v0.12.0-alpha.0` or whichever version introduces the feature) in
the schema versioning system, following the existing pattern used for
the `source` field addition in `v0.9.0-alpha.0`.

## Implementation

A suggested phased approach:

1. **Schema and parsing**: Add the `replace` field to `#Dep`.
   Extend `modfiledata.File` and `modfile.Parse` to handle
   `local-module.cue` alongside `module.cue`. Add strict validation
   rejecting `replace` fields in `module.cue` dep entries and
   rejecting `module` and `language` fields in `local-module.cue`.

2. **Directory replacement in the loader**: Teach the module loader
   to resolve directory-replaced modules from the local filesystem
   instead of a registry. Handle both relative and absolute paths,
   including the `io/fs.FS` boundary issues on Windows.

3. **Module-version replacement**: Extend the registry resolution
   layer to remap module paths and versions according to replace
   directives before fetching.

4. **`cue mod tidy` support**: Implement dual-graph resolution
   and the synchronization logic described above, including
   `--local-only` and automatic removal of empty `local-module.cue`
   files.

5. **Exclude directives** (future): Once replace is stable, exclude
   can be added following the same two-file pattern if needed, or
   placed solely in `local-module.cue`.
