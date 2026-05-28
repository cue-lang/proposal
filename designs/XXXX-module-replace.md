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
from local development configuration, making development-time
configuration explicit and easy to detect before publication.

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

It is worth being precise about what this achieves. Unlike `go.work`,
`local-module.cue` lives inside the module and is committed by
default (see "Why not `.gitignore` `local-module.cue` by default?"),
so it does not make it structurally impossible to commit a
machine-specific directory replace. What it does provide is a
dedicated, well-known home for development-time configuration:
replace directives are explicit, isolated from published metadata,
and easy to detect before publication (whether by a person reading a
diff or by tooling). Publication itself is structurally protected —
`local-module.cue` is omitted from the published module zip and the
Central Registry rejects any published module that contains replace
directives — so the goal is to make development-time configuration
explicit and catchable, not to make committing it impossible.

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

### Authoring `local-module.cue`

The canonical workflow is to author a *sparse* `local-module.cue`
that lists only the dependency (or dependencies) being replaced, and
to let `cue mod tidy` fill in the rest. For example, to point a
dependency at a local checkout, a developer writes:

```cue
// local-module.cue
deps: "example.com/foo@v0": {
    replace: "../local-foo"
}
```

and runs `cue mod tidy`. Everything else — the full main-module
dependency set and `default` annotations — is inherited from
`module.cue` via the merge described below, so there is no
duplication of the dependency graph to keep in sync by hand. This is
also the natural shape for tooling: an LSP code action such as "work
on this module locally" or "add a replace for this dependency" adds a
single entry, not a wholesale copy.

Both files share the same "lax" syntax (the existing `#File` schema
minus `#Strict` constraints), so a full copy of `module.cue` is also
a *valid* `local-module.cue`. This is occasionally a convenient
starting point when editing an existing `module.cue` by hand, and
`cue mod tidy` will normalize it (removing the `module` and
`language: version` fields, which belong exclusively in
`module.cue`). But the sparse overlay is the recommended form, and
the blessed workflow does not duplicate the dependency list.

If `cue mod tidy` finds an incompatibility between the two files, it
reports an error: a mismatched module path is a hard error; differing
language versions resolve to the greater of the two (with a warning).

#### How the two files combine

The merge between `local-module.cue` and `module.cue` is *not* a CUE
unification. For any dependency present in both files, the version is
reconciled to the maximum of the two under MVS (minimum version
selection) semantics, exactly as `cue mod tidy` does when resolving a
dependency graph — not unified (which would treat differing versions
as a conflict). This is what allows a sparse `local-module.cue` to
inherit unchanged versions from `module.cue` while overriding only
the entries it mentions.

A directory-replaced dependency is a special case: its content comes
from the local checkout, so its declared version is nominal. That
nominal version is recorded (it determines how the dependency is
resolved should the replace be removed) but it does not participate
in the max-version reconciliation against `module.cue` — the local
content is used regardless of which side declares the higher version.
Module-version replacements, by contrast, name a real registry
version that does participate in reconciliation in the usual way.

#### Omitting redundant versions

Because a dependency present in both files is reconciled to a single
version, repeating that version in `local-module.cue` would be pure
duplication. The `v` field is therefore optional in
`local-module.cue`: when omitted for a dependency that also appears in
`module.cue`, the version is taken from `module.cue`. A dependency
must still be *listed* in `local-module.cue` for it to be part of the
main-module view (the file is the complete main-module dependency
set, not a delta), but its version may be dropped.

`cue mod tidy` produces output in this minimal form: it omits the `v`
field from every `local-module.cue` entry whose version is identical
to that module's version in `module.cue`. After tidying, a typical
directory replace reduces to

```cue
// local-module.cue
deps: "example.com/foo@v0": {
    replace: "../local-foo"
}
```

with the version living only in `module.cue`. A dependency that is
listed only to complete the main-module view (neither replaced nor at
a different version) is written with no fields at all
(`"example.com/bar@v0": {}`). A version is retained only when it
differs from `module.cue` or when the module is absent from
`module.cue` entirely (for example a module that exists only as a
local checkout under `--local-only`), since there is then nothing to
inherit it from.

### Replace field syntax

Replacements are expressed as a `replace` field within individual
`deps` entries in `local-module.cue`. Two forms are supported. The
examples below show an explicit `v` field for clarity, but in
practice it is usually omitted and inherited from `module.cue` (see
"Omitting redundant versions"); that is the form `cue mod tidy`
writes.

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

Note that the replacement target need not share the module path of
the dependency it replaces: the example above redirects
`example.com/foo@v0` to the differently-named `example.com/bar`. In
other words, this directly supports module aliasing (the use case of
[golang/go#26904](https://github.com/golang/go/issues/26904)). This
necessarily works in CUE because a module published to a given path
is required, by construction, to declare that same path — both in the
Central Registry and (where enforced) in `cmd/cue` — so the
replacement target cannot itself re-declare the original module's
path the way Go's replace mechanism requires.

The `replace` value is interpreted as a directory path if it starts
with `.` or `/`, or if it matches a Windows absolute path (a drive
letter followed by a colon, e.g., `C:\libs\foo` or `C:/libs/foo`).
Otherwise it is interpreted as a module path with version. This
disambiguation is unambiguous because valid module paths cannot
start with `.` or `/` and cannot contain a colon (which appears in
the second character of a Windows drive-letter path).

### Schema changes

The `#Dep` type gains an optional `replace` field, and its version
field `v` becomes optional in the lax (`#File`) schema:

```cue
#Dep: {
    // v indicates the minimum required version of the module. It may
    // be null when the version is unknown and the entry is only
    // present to be replaced. In a cue.mod/local-module.cue file it
    // may also be omitted entirely for a dependency that is also
    // present in cue.mod/module.cue, in which case the version is
    // taken from there.
    v?: #Semver | null
    default?: bool

    // replace specifies a replacement for this dependency.
    // A value starting with "." or "/", or matching a Windows
    // absolute path (e.g. "C:\..." or "C:/..."), is a directory
    // path; otherwise it is a module path with version.
    replace?: string
}
```

Making `v` optional is what permits a sparse `local-module.cue` to
name a dependency by path alone and inherit its version from
`module.cue` (see "Omitting redundant versions" below). A dependency
that omits `v` and is *not* present in `module.cue` is only accepted
when it is a replace-only placeholder (it has a `replace` field and a
module path carrying its major version); otherwise it is an error,
because there is no version to resolve it against.

The `#Strict` schema (used at publish time) rejects the `replace`
field and continues to require a concrete version, so the relaxation
of `v` applies only to the lax main-module and `local-module.cue`
views:

```cue
#Strict: #File & {
    // ... existing strict constraints ...

    // A concrete version is required in published modules.
    #Dep: v!: #Semver

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

5. Versions in `local-module.cue` that are redundant with `module.cue`
   are omitted from the written file (see "Omitting redundant
   versions").

6. If no dep entry in `local-module.cue` contains a `replace` field
   after tidying, `cue mod tidy` removes the file entirely, since
   it serves no purpose without replacements.

#### Migrating a replace directive out of `module.cue`

A `replace` field is not valid in `module.cue`: loading a module
whose `module.cue` carries one is an error (and the registry rejects
it at publish time). As a convenience, however, `cue mod tidy` does
not reject such a file. Instead it treats the misplaced replace
directive as something to fix: it records the replacement in
`local-module.cue` (creating the file if necessary) and removes it
from `module.cue`. `cue mod tidy --check` reports the module as not
tidy in this state. This gives a smooth path for a developer who adds
a replace to the wrong file, or who is migrating an older module by
hand, without having to know up front which file the directive
belongs in.

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

Although a directory replace that points at a developer's personal,
machine-specific checkout is non-portable and a poor candidate for
version control, committing a directory replace is not always a
mistake. A project may deliberately rely on a fixed, repository-local
directory layout — for example a module that references a sibling
directory that is part of the same checkout and is not itself
intended to be published as a dependency. Committing the replace is
the correct behaviour there. Likewise, module-version replaces can be
useful to share across a team (e.g., pointing to a patched fork until
a fix is upstreamed).

Making the file always gitignored would prevent these legitimate
uses, so whether to commit `local-module.cue` is left to the
developer — a decision between them and their VCS, exactly as it
would be for any other source file. Publication is protected
separately and by construction (the file is omitted from the
published zip and rejected by the registry; see "Interaction with
registries"), so gitignoring is not needed to keep replace directives
out of published modules. Tooling can additionally warn when
`local-module.cue` contains a directory replace and is tracked by
git, so that an *accidentally* committed machine-specific path is
easy to notice without forbidding the deliberate case.

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

1. **Schema and parsing**: Add the `replace` field to `#Dep` and make
   `v` optional in the lax schema (keeping it required in `#Strict`).
   Extend `modfiledata.File` and `modfile.Parse` to handle
   `local-module.cue` alongside `module.cue`, including a `ParseLocal`
   that inherits identity from `module.cue` and fills in omitted
   versions from it. Reject `module`/`language` fields that disagree
   with `module.cue`. A `replace` field in `module.cue` is rejected at
   load and publish time, but tolerated and migrated by `cue mod tidy`
   (see "Migrating a replace directive out of `module.cue`").

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
