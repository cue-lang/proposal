# Proposal: Module Replace Directives with a Two-File Approach

Roger Peppe

Date: 2026-05-27

Discussion at https://github.com/cue-lang/cue/discussions/4389.

## Abstract

We propose adding `replaceWith` directive support to the CUE module
system, covering both directory replacements (for local checkouts)
and module-version replacements (for fork-based workflows). Rather
than placing replace directives in the existing `cue.mod/module.cue`
file, we introduce an optional companion file `cue.mod/local-module.cue`
that holds replace directives and an alternative dependency list.
This two-file design cleanly separates published module metadata
from main-module-only configuration — overrides that change how you
build without changing what your consumers see — making them explicit
and easy to detect before publication.

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

### Use cases

Replace directives serve several distinct user stories. They are
numbered here so that the rest of the document — in particular the open
question in "Aliasing and hidden fields" — can refer to them precisely
and be explicit about which are supported under a given design choice.

**UC1 — Coordinated local development ("edit two modules at once").**
This is the most pressing use case. A developer working on module A
that depends on module B needs to make coordinated changes to both.
Without replace directives, changes to B must be published (or at least
pushed to a registry) before they can be tested in A, creating a
painful publish-test-fix cycle. A *directory replace* points A's
dependency on B at a local checkout, so both can be edited and tested
together and published only when everything works.

**UC2 — Committed repository-local layout.** A project deliberately
relies on a fixed, repository-local directory layout — for example a
module that references a sibling directory that is part of the same
checkout and is not itself intended to be published as a dependency.
Here a directory replace is committed on purpose, rather than being a
machine-specific accident to be kept out of version control.

**UC3 — Patched or forked dependency.** A dependency has a bug and the
developer needs a patched fork at a specific version until the fix is
upstreamed. A *module-version replace* redirects the dependency to the
fork. This is often useful to share across a team.

**UC4 — Renaming a dependency to a differently-named module.** A
dependency's canonical module path has moved, and the whole module
graph is redirected to the new name. A module-version replace points
the old path at the new, differently-named module. Because the
replacement target is reached *only* through the replaced dependency —
no other part of the graph imports it directly under its own name —
there is a single package identity in play and no ambiguity.

**UC5 — Interoperating across a hard fork.** A widely-used module's
import path has migrated to a drop-in replacement, and the dependency
graph now contains consumers of *both*: some dependencies still import
the old path while others have adopted the new one. A replace is used
to make the two interoperate, so that values produced under the old
path can be used where the new path is expected, and vice versa. This
requires the old and new modules to be treated as the *same* package.
Unlike UC4, the replacement target is also imported directly elsewhere
in the configuration. This use case is the subject of the open question
in "Aliasing and hidden fields", and is the one whose support is not
yet decided.

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
and main-module-only metadata.

The defining property of `local-module.cue` is scope: its contents
are honoured only when the module is the main module and are ignored
entirely when the module is consumed as a dependency. In this sense
"local" in the filename means "local to this module — not propagated
to consumers", not "machine-local". Replace directives change how
you build without affecting what your consumers see; that is the
core guarantee. "Local development" is the most common motivation,
but as UC2–UC4 show, replaces are often committed, shared, and
persistent. `local-module.cue` provides a dedicated, well-known
home for all such overrides: they are explicit, isolated from
published metadata, and easy to detect before publication.
Publication is structurally protected separately — `local-module.cue`
is omitted from the published module zip and the Central Registry
rejects any published module that contains replace directives.

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
  include a `replaceWith` field specifying the replacement target.
- The `default` annotations that correspond to the main-module
  dependency set.

Only `module.cue` contains the module path and language version.
Only dep entries in `local-module.cue` may contain `replaceWith` fields.

When `local-module.cue` is absent, the module system behaves exactly
as it does today: `module.cue` is the sole source of truth.

When `local-module.cue` is present, the CUE tooling merges
information from both files. For dependency resolution in main-module
mode, the dependencies in `local-module.cue` take precedence, with
replace directives applied. For publishing or when the module is
used as a dependency, only `module.cue` is consulted: a
`local-module.cue` belonging to a module that is itself consumed as a
dependency is ignored entirely, so replace directives are honoured
only in the main module and never affect downstream consumers.

### Authoring `local-module.cue`

The canonical workflow is to author a *sparse* `local-module.cue`
that lists only the dependency (or dependencies) being replaced, and
to let `cue mod tidy` fill in the rest. For example, to point a
dependency at a local checkout, a developer writes:

```cue
// local-module.cue
deps: "example.com/foo@v0": {
    replaceWith: "../local-foo"
}
```

and runs `cue mod tidy`. Everything else — the full main-module
dependency set and `default` annotations — is inherited from
`module.cue` via the merge described below, so there is no
duplication of the dependency graph to keep in sync by hand. This is
also the natural shape for tooling: an LSP code action such as "work
on this module locally" or "add a replace for this dependency" adds a
single entry, not a wholesale copy.

In practice the most common way to produce that sparse file is not to
create it by hand at all. Starting from a module that has no
`local-module.cue`, a developer adds a `replaceWith` field to the relevant
dependency in the `module.cue` they already have and runs
`cue mod tidy`. A `replaceWith` field is not itself valid in `module.cue`,
but rather than rejecting it `cue mod tidy` takes it as the signal to
set things up: it removes the directive from `module.cue` and writes an
appropriate `local-module.cue` containing it (see "Migrating a replace
directive out of `module.cue`"). From then on the developer edits
`local-module.cue` directly.

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
    replaceWith: "../local-foo"
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

Replacements are expressed as a `replaceWith` field within individual
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
    replaceWith: "../local-foo"
}
```

Both relative and absolute paths are supported. Relative paths are
resolved relative to the directory containing `cue.mod`. On Windows,
absolute paths with drive letters are recognized:

```cue
deps: "example.com/foo@v0": {
    v: "v0.0.1"
    replaceWith: "C:/Users/dev/local-foo"
}
```

**Module-version replacement** redirects a module to a different
module and version in the registry:

```cue
// local-module.cue
deps: "example.com/foo@v0": {
    v: "v0.0.1"
    replaceWith: "example.com/bar@v0.1.2"
}
```

The replacement module is fetched from the registry in the usual
way. The replacement target is a full module path with version,
indicating the specific version to use.

Note that the replacement target need not share the module path of
the dependency it replaces: the example above redirects
`example.com/foo@v0` to the differently-named `example.com/bar`. This
supports renaming a dependency to a differently-named module (UC4) —
the module-aliasing capability requested in
[golang/go#26904](https://github.com/golang/go/issues/26904). It works
because a module published to a given path is required, by
construction, to declare that same path — both in the Central Registry
and (where enforced) in `cmd/cue` — so the replacement target cannot
itself re-declare the original module's path.

Redirecting to a differently-named target is unambiguous as long as
that target is reached only through the replacement (UC4): there is a
single package identity in play. It becomes a harder question when the
target is *also* imported directly elsewhere in the same configuration,
so that the replaced module and its target must interoperate as one
package (UC5). That case interacts with CUE's per-package hidden-field
namespaces and is the subject of "Open questions: aliasing and hidden
fields" below.

The `replaceWith` value is interpreted as a directory path if it starts
with `.` or `/`, or if it matches a Windows absolute path (a drive
letter followed by a colon, e.g., `C:\libs\foo` or `C:/libs/foo`).
Otherwise it is interpreted as a module path with version. This
disambiguation is unambiguous because valid module paths cannot
start with `.` or `/` and cannot contain a colon (which appears in
the second character of a Windows drive-letter path).

### Schema changes

The `#Dep` type gains an optional `replaceWith` field, and its version
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

    // replaceWith specifies a replacement for this dependency.
    // A value starting with "." or "/", or matching a Windows
    // absolute path (e.g. "C:\..." or "C:/..."), is a directory
    // path; otherwise it is a module path with version.
    replaceWith?: string
}
```

Making `v` optional is what permits a sparse `local-module.cue` to
name a dependency by path alone and inherit its version from
`module.cue` (see "Omitting redundant versions" below). A dependency
that omits `v` and is *not* present in `module.cue` is only accepted
when it is a replace-only placeholder (it has a `replaceWith` field and a
module path carrying its major version); otherwise it is an error,
because there is no version to resolve it against.

The `#Strict` schema (used at publish time) rejects the `replaceWith`
field and continues to require a concrete version, so the relaxation
of `v` applies only to the lax main-module and `local-module.cue`
views:

```cue
#Strict: #File & {
    // ... existing strict constraints ...

    // A concrete version is required in published modules.
    #Dep: v!: #Semver

    // Replacements are not permitted in published modules.
    #Dep: replaceWith?: _errorReplaceNotPermittedInStrict
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

6. If no dep entry in `local-module.cue` contains a `replaceWith` field
   after tidying, `cue mod tidy` removes the file entirely, since
   it serves no purpose without replacements.

#### Migrating a replace directive out of `module.cue`

A `replaceWith` field is not valid in `module.cue`: loading a module
whose `module.cue` carries one is an error (and the registry rejects
it at publish time). As a convenience, however, `cue mod tidy` does
not reject such a file. Instead it treats the misplaced replace
directive as something to fix: it records the replacement in
`local-module.cue` (creating the file if necessary) and removes it
from `module.cue`. `cue mod tidy --check` reports the module as not
tidy in this state.

This is in fact the most common way a `local-module.cue` comes into
existence (see "Authoring `local-module.cue`"): a developer adds a
`replaceWith` to the `module.cue` they already have, and `cue mod tidy`
relocates it into a new `local-module.cue`, which the developer is
then free to edit. The same path also smoothly handles a developer
who is migrating an older module by hand, without their having to know
up front which file the directive belongs in.

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
because it contains main-module-only configuration that is
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
mistake. UC2 above is exactly such a case: a project may deliberately
rely on a fixed, repository-local directory layout — for example a
module that references a sibling directory that is part of the same
checkout and is not itself intended to be published as a dependency.
Committing the replace is the correct behaviour there. Likewise, the
module-version replaces of UC3 and UC4 can be useful to share across a
team (e.g., pointing to a patched fork until a fix is upstreamed).

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

### What about `cue export $pkg@$version` ?

The `cue` command line supports referring to a package at an absolute
version. As it is the only package inside the build it is possible we
could treat its module as the main module and apply replace
directives. However, by implication from the rest of the proposal, in
general we do not support having `local-module.cue` inside
dependencies, and hence we will not recognize it when found in such a
dependency. In fact, we will reject downloaded modules when they
contain a `cue.mod/local-module.cue` file (this should not happen
unless someone is publishing modules to a custom registry without using
`cue mod publish`), providing defense in depth against potentially
malicious registry contents.

## Open questions

### Aliasing and hidden fields

Of the use cases above, UC5 — interoperating across a hard fork — is
the one whose support is genuinely in question. The scope of the
question is narrow, and worth stating plainly: this is *not* about
disallowing replacement to a differently-named module in general.
Redirecting a dependency to a renamed or forked module (UC3 and UC4)
remains supported, because there the replacement target is reached only
through the replacement and therefore has a single package identity.
What is in question is specifically whether the old and new modules can
be aliased so that consumers of the old import path and consumers of
the new one *interoperate* within a single configuration (UC5).

It has been suggested that a package within a replacement target should
be importable only through the dependency it replaces — that is, if
`example.com/foo` is replaced by `example.com/bar`, then
`example.com/bar` should not also be importable directly elsewhere in
the same configuration. Under that rule UC1–UC4 all work and only UC5
is rejected. The reasoning is that allowing both leaves the two
packages distinct, whereas making them interoperate through the replace
amounts to treating them as the *same* package, and CUE's per-package
hidden-field namespaces make the difference between "distinct" and "the
same" observable.

The concern is about hidden fields. A hidden field (e.g. `_foo`) is
scoped to the package that declares it, so two values are considered to
share a hidden field only if they come from the same package. Treating
`foo` and `bar` as aliases (UC5) means treating values drawn from the
two packages as interchangeable. But two packages that are *already*
both imported cannot always be safely equated after the fact: if a
configuration imports both, holds a value `A` from one and `B` from the
other, and unifies them somewhere (`A.x & B.x`), then retroactively
equating the packages can change the meaning of — or invalidate —
hidden fields that were previously in distinct namespaces.

Against disallowing UC5, it is a realistic and probably common
situation, and the parts of the graph that have switched to the new
path genuinely need to interoperate with the parts that have not — for
example, where a value produced by an old-path dependency is passed to
a new-path dependency that expects it. It would seem reasonable, too,
for the old module to become a thin forwarding module to the new one
without that counting as a breaking change, and that is essentially the
same operation. On this view UC5 is more useful, and more likely in
practice, than a configuration that imports the replaced module and its
target simultaneously and relies on their hidden-field namespaces
staying distinct.

The difficulty is that the two goals appear to be in direct opposition.
If the dependencies reached through a replacement are kept distinct
from the rest of the configuration, they cannot interoperate with
dependencies outside it, because their hidden fields live in a separate
namespace — which defeats UC5. If they are not kept distinct, the
collision problem above can arise. Resolving this tension — including
deciding whether the initial implementation should simply disallow UC5
(rejecting a replacement whose target is also imported directly under
its own name) until the semantics are settled, while keeping UC1–UC4 —
is left to the design discussion.

## Compatibility

This proposal is fully backward compatible. Existing modules that
have no `local-module.cue` file behave identically to the current
system. The schema changes are additive: the `replaceWith` field on
`#Dep` is optional and only permitted in `local-module.cue`.

The proposal requires a new language version gate (e.g.,
`v0.12.0-alpha.0` or whichever version introduces the feature) in
the schema versioning system, following the existing pattern used for
the `source` field addition in `v0.9.0-alpha.0`.

## Implementation

A suggested phased approach:

1. **Schema and parsing**: Add the `replaceWith` field to `#Dep` and make
   `v` optional in the lax schema (keeping it required in `#Strict`).
   Extend `modfiledata.File` and `modfile.Parse` to handle
   `local-module.cue` alongside `module.cue`, including a `ParseLocal`
   that inherits identity from `module.cue` and fills in omitted
   versions from it. Reject `module`/`language` fields that disagree
   with `module.cue`. A `replaceWith` field in `module.cue` is rejected at
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
