# Background

There is a big demand from the CUE community to allow sharing CUE code within a
broader ecosystem, much like the package managers that are available for
programming languages or schema stores.

To address this demand, we propose the creation of a CUE registry that will
serve as a central repository for CUE modules. A module is a collection of
packages that are released, versioned, and distributed together. The registry
will allow users to share and discover CUE modules and will provide versioning
and dependency management features.

Non-goals for this project include replacing existing package managers or schema
stores, or providing a full-fledged dependency management solution for all
programming languages. That said, we will aim for a design that is compatible
with supporting co-versioning with other languages in the future.

The registry will be implemented as a web service. Users will typically
interface with this web service through the `cue` command line or integrations
with code hosting sites such as a GitHub app.

*Note: CUE, or API definitions in general, seem to exhibit a somewhat different
life cycle from a typical programming language. We have observed it is more
common to bump major versions of configuration, for instance. Also, there are
more opportunities for analyzing restricted languages like CUE, and thus
enforcing certain properties, than for general purpose languages.*

# Overview

We propose a modules ecosystem approach for CUE consisting of the following
components:

- A central registry service that allows authors to publish both public and
  private modules.
- Modification of the `cue` tool so that it is capable of resolving module
  dependencies in a predictable and consistent manner, as well as downloading
  them.
- A GitHub app that authors can use as a convenience for publishing
  GitHub-hosted modules to the registry.
- Use of Semantic Versioning for modules.
- Standardized module identifiers for major versions.

Each of these design aspects is motivated and explained in more detail in the
next section.

# Detailed Design

Although the design of CUE modules is heavily influenced by the Go modules
design, there are many aspects where it deviates, incorporating lessons learned,
but also taking into account the different use case of CUE versus Go.  In this
section we discuss some of the details along with motivations for the design.

## Registry

### Rationale

Most schema stores and package managers for programming languages follow follow
a registry design. For example, Rust with [crates.io](https://crates.io/) and
JavaScript with [NPM](https://www.npmjs.com/).

Under such a design, a module _author_ _publishes_ a version of a module to a
registry. Module _consumers_ or _users_ then _discover_, _resolve_, and
_download_ dependencies via the registry.

Go deviates from this approach by using a direct approach. Users use a Version
Control System (VCS) to publish modules to a namespace of their choice, combined
with a [proxy](https://proxy.golang.org) to improve speed and reliability.

Advantages of using a registry:

- No need to support VCS integrations in the client
    - Overall greater simplicity and simplified security concerns.
    - There is a clear, simple contract against which VCS integrations can be
      implemented.
    - As CUE focuses on configuration, it is more likely to be evaluated in a
      production setting, rather than on a developer's machine. Reliance on VCS
      complicates matters considerably.
- Performance:
    - Cloning repositories can be costly. Under a VCS-based module design, long
      download times and unreliable behaviour can result when use of a proxy is
      not an option. For example, private modules present a challenge in Go
      because they are not hosted in the public proxy. Choosing a design that is
      not VCS-based sidesteps such challenges.
- Enhanced security:
    - Reduced attack surface, because VCS interaction is taken out of the
      equation.
    - Allows guaranteeing the existence of module dependencies.
- Design allows for private modules.
- Discovery of modules is a feature that naturally follows a user's interactions
  with the registry.
- Possibility for enhanced functionality:
    - Features like cross-validation become possible with this active "publish"
      step by the author.

The historical context that led to the choice of the direct, VCS-based design
for Go does not apply to CUE. It seems likely that Go would have opted for a
registry design if the historical context would have permitted it.

The CUE tooling will allow user-provided registries. This allows users, for
instance, to have an in-house registry for private modules.

### Security

The registry server will use authorization for private modules. This feature
will come later and will be discussed in a separate document.

The registry will supply cryptographic content hashes for modules. Modules
themselves will contain hashes of their dependencies. This will allow users to
verify module contents even in the presence of untrusted registries.

## Versioning

### Semantic Versioning (SemVer)

We propose using [SemVer v2](https://semver.org/spec/v2.0.0.html) versioning.
The main reason for this choice is to use a familiar standard. While SemVer is
not perfect, it meets our needs and is widely used.

Our main requirement was a versioning scheme that clearly distinguishes major
changes from other changes. SemVer meets this requirement. Its distinction
between minor and patch versions can be useful for schema.

The fact that modules are always published with an explicit version eliminates
the need for complicated
[pseudo-versions](https://go.dev/ref/mod#pseudo-versions). However, pre-release
conventions like `-beta` and `-alpha` can still be used when appropriate.

## Module and Package Paths

A CUE module currently defines a domain name-based path that is used as a prefix
for any of the packages contained within it. We will continue to use this
approach for identifying modules.

For example, this will allow modules with paths `github.com/my/pkg@v1` or
`my.domain/pkg@v2` to be published to the CUE registry.

### Canonical Module Path

The module path is defined in the `module` field in `cue.mod/module.cue`. Each
module is uniquely identified by the module path and major version:
`doma.in/path/to/module/root@v1`.

Explicitly defining the major version in the `module.cue` file allows for better
automated validation during publishing and prevents accidents. We plan to
provide convenience functionality in the `cue` CLI  for changing the major
version of imported modules.

### Rationale

Motivation for a domain-name-based approach:

- Backwards compatible
    - CUE currently already uses the domain name-based approach for imports, so
      this will remain consistent.
- Publisher identity guarantees
    - Publishers will need to prove ownership of the namespace they are
      publishing to. This means a package path gives some guarantees that the
      publisher is indeed a trusted source. Proving publisher ownership via
      domain names prevents common attestation- and provenance-related supply
      chain issues.
- Many package managers use a `$user/$pkgname` or `$pkgname` approach to
  defining the namespace. We find the resulting land-grab an undesirable
  phenomenon, and one that continues to be a common abuse pattern.

Motivation for including the major version in the module path:

- There is a single way of marking a major version and a single module path to
  identify all major versions.
    - For instance, for Go modules, `example.com/foo/v1` and
      `example.com/foo/v2` may or may not be different major versions of the
      same module, since  `/v1`  and  `/v2`  are also valid sub-packages. In
      CUE, these would always be different modules (because major versions in
      CUE are not specified with regular path elements). For the interpretation
      of two different major versions of the same module, the corresponding CUE
      identifiers would look like `example.com/foo@v1` and `example.com/foo@v2`.
- Go special-cases `/vx` directories to allow different major modules to share
  the same path in a VCS repo. The resulting ambiguities introduce complexity
  and confusion. The proposed approach avoids this.
- This approach does not prescribe or limit how the author represents different
  major versions of a module in VCS.
- This approach allows future support for more precise version imports or even
  importing multiple non-major versions at once.
    - This could be useful when computing schemas that represent a fleet of
      server API versions.
    - Some users have expressed interest in having more accurate pinning than
      major version.

### Disallowing module nesting (for now)

Currently, we will not allow nested module identifiers. That is, the registry
will not allowed to have both module `example.com/foo` and
`example.com/foo/bar`. Down the line we will probably allow this to support
module “splitting”.

### Imports

CUE package imports need to be able to specify the major version of a module. We
propose allowing this by specifying the major version at *the end* of the
package import path.

```go
import "doma.in/path/to/module/root/mypkg@v1"
```

A big advantage of specifying the major version at the end of the path is that
it allows for splitting up modules into smaller pieces while remaining backwards
compatible with existing import paths.

An import statement that has no major version will use the major version implied
from the module’s major version declared inside the `cue.mod/module.cue` file.
This has the advantage that it makes it easier to update a major version of a
module dependency across a large set of configuration files. Considering that
bumping major versions seems more common in configuration land, this can help
productivity considerably.

The `cue.mod/module.cue` file will need to be able to indicate a default version
in case the CUE code depends on two major versions of a given module.

For a newly added module, the tooling will choose the latest major version that
has a non-prerelease version.

## Backwards Compatibility

As CUE allows defining schema, data, validation, and templating, among other
things, it may be unclear what backwards compatibility means. In general, it is
hard to define backwards compatibility on anything other than schema.

We propose therefore to only support backwards compatibility checks for
definitions (fields starting with a `#`).

### Compatibility check

The compatibility check is applied to any subsequent non-major version.

A configuration `B` is said to backwards compatible with `A` if for all `A.#X`,
there exists a `B.#X` for which it holds that `A.#X` is an instance of `B.#X`
(in other words, `B.#X`
[subsumes](https://tip.cuelang.org/docs/references/spec/#field-constraints)
`A.#X`). For this purpose, it is assumed that any undefined field `x` in either
`A.#X` or `B.#X` is defined as `x?: _|_`.

In simpler terms, this means that:

- A newer version may not remove a definition from an older version.
- A new version may add fields to a definition, but not remove them.
- A regular field may become a required field, and a required field may become
  optional, but not the other way around. _See the [required fields
  proposal](https://github.com/cue-lang/proposal/blob/main/designs/1951-required-fields-v2.md)
  for an explanation of required fields._
- A field value for `B.#X.foo` may relax, but not tighten, constraints compared
  to `A.#X.foo`.

The general idea here is that data, templates, and policy inherently change in
ways that cannot easily be expressed in relationships in terms of the CUE value
lattice. Moreover, it seems that it is generally more appropriate to enforce
rules for proper usage of templates, policy, etc. on the client side.

### Module authors responsibilities

Module authors need to be aware of the backwards compatibility rules, as these
may influence design decisions.

As a general rule of thumb, pure schema (no defaults, no regular fields) should
be exported as `#Foo` while, for instance, an equivalent schema with recommended
default values can be provided as `Foo: #Foo & { // defaults }`.  Organizing
schema this way allows module authors to change default values without breaking
compatibility rules.

Nothing stops module authors, of course, from providing templates as
definitions, thereby guaranteeing that default values will not change. For
instance, consider the following enumeration

```go
Levels: "high" | "medium" | "low"
```

Because `Levels` is not a definition, subsequent versions could drop `"medium"`
from the enumeration. A module author, however, could opt to instead define the
above as

```go
#Levels: "high" | "medium" | "low"
```

thereby giving the guarantee that the list of enumerations can only grow.

### Module consumers' responsibilities

Compatibility guarantees only go so far. Module consumers should remain cautious
when using schema for which they do not control changes. Consider this example

```go
import "example.com/third/party"

Foo: {
	party.#Schema

	newField: int
}
```

where `newField` is not defined in `party.#Schema`. Under the compatibility
rules, newer versions of `party.#Schema` may add a `newField?: string`. This
would break the author of `Foo`.

Another example:

```go
import "example.com/third/party"

Foo: party.#Schema
Foo: existingField: _
```

Assume `party.#Schema` defines `existingField?: 5`. `Foo` is valid and
exportable, as `existingField` will result in a concrete value. However, at a
later stage, the module publisher could change this to `existingField?: 5 | 6`,
a valid change, but possibly breaking the user’s configuration as exporting it
may now result in an incomplete error.

Similarly, users should also be cognizant when using templates:

```go
import "example.com/third/party"

Foo: party.IntValue
Foo: 6
```

Given a `party.IntValue` of `6`, this will pass. However, the module publisher
is allowed to change this under the compatibility rules (note it is `IntValue`,
not `#IntValue`). Changing it to `7`, for instance, would break this user.

In all these breakage scenarios, we consider it the responsibility of the user
of a module to avoid or deal with breakage. We envision having usage guidelines
as well as tooling support to avoid such pitfalls.

### Exceptions to the compatibility check

Under some circumstances it may be desirable to publish a newer version of a
module that is not backwards compatible. Consider the following schema:

```go
// myserver.cue
#Request: {
	gauge: >=0 & <=1
}
```

This schema allows the value of `1` for `gauge`. However, suppose the author
inadvertently made a mistake and the server does not actually support a `gauge`
value of `1`; a request is guaranteed to fail for such values.

A fix is made accordingly:

```go
// myserver.cue
#Request: {
	gauge: >=0 & <1
}
```

Pushing this using the same major version is a violation of the compatibility
rules. However, not doing so may result in uncaught bugs and may even pose a
security issue.

The CUE tooling should therefore allow for a  process by which the compatibility
rule can be bypassed.

We are aware of this need and are still figuring out how to best go about this.

### Comparison to wire compatibility

Note that the CUE backwards compatibility rules do not ensure wire
compatibility. Most notably, the rules allow newer versions to turn required
fields into optional. Using a newer CUE schema to validate a message designated
for an older server could therefore possibly not catch an erroneous omission.

There are several workarounds for this we could consider in the future:

- allow module publishers to opt into stricter compatibility rules;
- provide “virtual” computed schemas that reflect best practices for a range of
  versions;
- validation rules that are aware of the differences with older versions.

### Incremental backwards compatibility

SemVer allows backwards incompatible changes in `v0` mode to allow for an API to
evolve until it is stable and can be moved to `v1`. One lesson learned from Go
modules is that it is often onerous to move to `v1` as there are always
uncertain aspects of the API. As a result, many modules remain in `v0`
indefinitely. Additionally, the same flexibility does not exist for `v2`, even
though this may be desirable.

To address this issue, we propose supporting annotations in CUE using the `@api`
attribute. This would allow different parts of the API to be marked as a
different state of maturity, providing backwards compatibility checking for
stable parts while still allowing experimental parts of the API to coexist.

While this is outside the scope of this design document, we believe it is an
important consideration for an ecosystem that relies on backwards compatibility
guarantees.

## The `cue.mod/module.cue` file

Module dependencies are configured in this file. It has the format below. Note
that there is no package clause. This is deliberate. All module-related data
must be specified in this single file.

_Also note that this schema relies on [required
fields](https://github.com/cue-ang/cue/discussions/1951). Required fields are
implemented as of CUE
[`v0.6.0-0.dev.0.20230322121530-0b681f5d193a`](https://github.com/cue-lang/cue/commit/0b681f5d193a55893fdec8386f4a72a6effcebb1)
and will be release in `v0.6.0-alpha.1` shortly._

Initially we will require this to be a data-only file.

```go
// This schema constrains a module.cue file. This form constrains module.cue files
// outside of the main module. For the module.cue file in a main module, the schema
// is less restrictive, because wherever #SemVer is used, a less specific version may be
// used instead, which will be rewritten to the canonical form by the cue command tooling.

// module indicates the module's path.
module!: #Module

// lang indicates the language version used by the code
// in this module - the minimum version of CUE required
// to evaluate the code in this module. When a later version of CUE
// is evaluating code in this module, this will be used to
// choose version-specific behavior. If an earlier version of CUE
// is used, an error will be given.
cue?: lang?: #SemVer

// description describes the purpose of this module.
description?: string

// When present, deprecated indicates that the module
// is deprecated and includes information about that deprecation, ideally
// mentioning an alternative that can be used instead.
deprecated?: string

// deps holds dependency information for modules, keyed by module path.
deps?: [#Module]: {
	// replace and replaceAll are mutually exclusive.
	mustexist(<=1, replace, replaceAll)

	// There must be at least one field specified for a given module.
	mustexist(>=1, v, exclude, replace, replaceAll)

	// v indicates the minimum required version of the module.
	// This can be null if the version is unknown and the module
	// entry is only present to be replaced.
	v!: #SemVer | null

	// default indicates this module is used as a default in case
	// more than one major version is specified for the same module
	// path. Imports must specify the exact major version for a
	// module path if there is more than one major version for that
	// path and default is not set for exactly one of them.
	default?: bool

	// exclude excludes a set of versions of the module.
	exclude?: [#SemVer]: true

	// replace specifies replacements for specific versions of
	// the module. This field is exclusive with replaceAll.
	replace?: [#SemVer]: #Replacement

	// replaceAll specifies a replacement for all versions of the module.
	// This field is exclusive with replace.
	replaceAll?: #Replacement
}

// The publish section can be used to restrict the scope of a module to prevent
// accidental publishing. This cannot be overridden on the command line.
// A published module cannot widen the scope of what is reported here.
publish?: {
	// Define the scope that is allowed by default.
	allow!: #Scope

	// default overrides the default scope that is used on the command line.
	default?: #Scope
}

#Scope: *"private" | "public"

// #RetractedVersion specifies either a single version
// to retract, or an inclusive range of versions to retract.
#RetractedVersion: #SemVer | {
	from!: #SemVer
	// TODO constrain to to be after from?
	to!: #SemVer
}

// #Replacement specifies a replacement for a module. It can either
// be a reference to a local directory or an alternative module with associated
// version.
#Replacement: #LocalPath | {
	m!: #Module
	v!: #SemVer
}

// #LocalPath constrains a filesystem path used for a module replacement,
// which must be either explicitly relative to the current directory or root-based.
#LocalPath: =~"^(./|../|/)"

// #Module constrains a module path.
// The major version indicator is optional, but should always be present
// in a normalized module.cue file.
// TODO encode the module path rules as regexp:
// WIP: (([\-_~a-zA-Z0-9][.\-_~a-zA-Z0-9]*[\-_~a-zA-Z0-9])|([\-_~a-zA-Z0-9]))(/([\-_~a-zA-Z0-9][.\-_~a-zA-Z0-9]*[\-_~a-zA-Z0-9])|([\-_~a-zA-Z0-9]))*
#Module: =~#"^[^@]+(@v(0|[1-9]\d*))?$"#

// #SemVer constrains a semantic version. This regular expression is taken from
// https://semver.org/spec/v2.0.0.html, but includes a mandatory initial "v".
#SemVer: =~#"^v(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$"#
```

### `sum.cue` file

We will likely support a `cue.mod/sum.cue` file analogous to the
[`go.sum`](https://go.dev/ref/mod#go-sum-files) file. Format as yet to be
defined.

## `cue` commands extensions

Here we discuss the initial set of supported commands. The first to be
implemented are `init`, `tidy`, and `publish`. Others will follow later.

### main module

### module cache

We will maintain a module cache in
[`os.UserCacheDir()/cuelang`](https://pkg.go.dev/os#UserCacheDir).

Files are stored as read-only in the module cache.

### version queries

### `cue mod init`

The `init` command will remain as is. The module name may now contain a major
version suffix. If a major version suffix is not supplied, an `@v0` suffix will
be added.


### `cue mod tidy`

Usage:

```
cue mod tidy [-i] [-v] [-cue=version] [-compat=version]
```

`cue mod tidy` ensures that the `cue.mod/module.cue` file matches the CUE code
in the module. It adds any missing module requirements necessary to build the
current module's packages and dependencies, and it removes requirements on
modules that don't provide any relevant packages. It also adds any missing
entries to `cue.sum` and removes unnecessary entries.

The `-i` flag causes `cue mod tidy` to attempt to proceed despite errors
encountered while loading packages.

The `-v` flag causes `cue mod tidy` to print information about removed modules
to standard error.

`cue mod tidy` works by loading all of the packages in the [main
module](#main-module) and all of the packages they import, recursively. `cue mod
tidy` acts as if all build tags are enabled, so it will mostly ignore `@if()`
attributes, even if those source files wouldn't normally be built. The one
exception is `@if(ignore)`, which will always be ignored. `cue mod tidy` will
not consider packages in the [main module](#main-module) in directories
named `testdata` or with names that start with `.` or `_` unless those packages
are explicitly imported by other packages.

Once `cue mod tidy` has loaded this set of packages, it ensures that each module
that provides one or more packages has an entry in `deps` in the [main
module's](#main-module#main-module#main-module) `cue.mod/module.cue`. `cue mod
tidy` will add a requirement on the latest version of each missing module (see
[version queries](#version-queries) for the definition of
the `latest` version). `cue mod tidy` will remove dependencies for modules that
don't provide any packages in the set described above.

`cue mod tidy` may also add or remove `@indirect()` attributes on entries in
`deps`. An `@indirect()` attribute denotes a module that does not provide a
package imported by a package in the [main module](#main-module). (See the
`deps` field in [The `cue.mod/module.cue` file](#the-cuemodmodulecue-file) for
more detail on when `@indirect()` dependencies and comments are added.)

If the `-cue` flag is set, `cue mod tidy` will update the `cue` field to the
indicated version.

### `cue mod publish`

Usage:

```
cue mod publish [-n] [--scope=version] [version]
```

Example:

```
$ cue mod publish
$ cue mod publish @v1.1.2
```

The `cue mod publish` command uploads the [main module](#main-module) to the
registry and tags it with the given version. If no version is given, it selects
the next available minor or patch release. The heuristics as to what constitutes
a major vs minor vs patch release are closely related to the concept of
[backwards compatibility](#backwards-compatibility) and will be more precisely
defined when this command is implemented (see [Implementation
Plan](#implementation-plan)). Broadly however:

* A major version increment is _required_ when a version is not backwards
  compatible (with respect to schema).
* A minor version increment is _preferred_ when a version is strictly subsumed
  by an older release, (with respect to schema).
* A patch version increment is _preferred_ when a new version is identical to
  previous version (with respect to schema).

The `-n` flag causes `publish` to do a dry run and report all errors without
actually changing the registry.

The `--scope` flag sets the visibility for the package, which may not be more
permissive than the scope set in the module file. The default scope is private.

### `cue mod upload`

Usage:

```
cue mod upload
```

The `cue mod upload` command uploads the [main module](#main-module) to the
registry and reports a unique identifier for this module.

### `cue mod release`

Usage:

```
cue mod release [-n] [--scope=domain] <id>[@<version>]
```

Example:

```
$ cue mod release ab34ab342a342bf
$ cue mod release ab34ab342a342bf@v1.3.1
$ cue mod release example.com/foo@v1.2.3 --scope=public
```

The `cue mod release` command assigns a version to a previously uploaded module
version. If no version is given, it selects the next available minor or patch
release. This command can also be used to change the scope of package, although
public packages cannot be made private or removed.

The `-n` flag causes `release` to do a dry run and report all errors without
actually changing the registry.

The `--scope` flag sets the visibility for the package, which may not be more
permissive than the scope set in the module file. The default scope is private.

### `cue mod download`

Usage:

```
cue mod download [-x] [modules]
```

Example:

```
$ cue mod download
$ cue mod download github.com/foo/bar/mod@v0.2.0
```

The `cue mod download` command downloads the named modules into the module
cache. Arguments can be module paths or module patterns selecting dependencies
of the [main module](#main-module) or version queries of the
form `path@version`. With no arguments, `download` applies to all dependencies
of the main module.

The `cue` command will automatically download modules as needed during ordinary
execution. The `cue mod download` command is useful mainly for pre-filling the
[module cache](#module-cache).

By default, `download` writes nothing to standard output. It prints progress
messages and errors to standard error.

### `cue mod vendor`

Usage:

```
cue mod vendor [-i] [-v] [-o]
```

The `cue mod vendor` command constructs a directory named `cue.mod/vendor` in
the [main module](#main-module)`s root directory that contains copies of all
packages needed to support evaluations and tests of packages in the [main
module](#main-module).

When vendoring is enabled, the `cue` command will load packages from
the `vendor` directory instead of downloading modules from their sources into
the [module cache](#module-cache) and using packages those downloaded copies.

`cue mod vendor` also creates the file `cue.mod/vendor/modules.cue` that
contains a list of vendored packages and the module versions they were copied
from.  When the `cue` command reads `vendor/modules.txt`, it checks that the
module versions are consistent with `cue.mod/module.cue`.
If `cue.mod/module.cue` changed since `vendor/modules.txt` was generated, `cue
mod vendor` should be run again.

Note that `cue mod vendor` removes the `vendor` directory if it exists before
re-constructing it. Local changes should not be made to vendored packages.
The `cue` command does not check that packages in the `vendor` directory have
not been modified, but one can verify the integrity of the `vendor` directory by
running `cue mod vendor` and checking that no changes were made.

The `-i` flag causes `cue mod vendor` to attempt to proceed despite errors
encountered while loading packages.

The `-v` flag causes `cue mod vendor` to print the names of vendored modules and
packages to standard error.

The `-o` flag causes `cue mod vendor` to output the vendor tree at the specified
directory instead of `vendor`. The argument can be either an absolute path or a
path relative to the module root.

The `vendor` command is expected to be expanded in the future to support
co-versioning of CUE with other package managers.

### `cue mod clean`

Removes all files in the [module cache](#module-cache).

### `cue mod edit`

This command will provide command-line-driven editing of the module.cue file,
with a similar design to Go's `go mod edit`. The exact design remains to be
specified.

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

## Minimum Version Selection

We plan to use [Minimum Version Selection
(MVS)](https://go.dev/ref/mod#minimal-version-selection) for resolving
dependency versions.

### Rationale

- MVS results in stable behavior when recomputing dependencies, resulting in the
  minimum set of required changes to meet new requirements. This fits well with
  the general properties desirable for configurations.
- MVS makes it easier to support co-versioning of CUE schemas that are stored
  along code in package managers of other languages.
- Compared to a SAT solver (as required by most more general versioning
  schemes): MVS has much more predictable running times.
- We considered using an “always on tip/latest” approach.
    - With the CUE registry it is possible to analyze the impact of schema
      changes, possibly making this manageable. We recognize, though, that many
      users want something more predictable.
    - An always on HEAD approach can still be simulated with MVS.

# Implementation Plan

At this stage, the first two phases of our implementation plan look like this:

* Phase 1:
  * Implement initial version of the registry.
  * Implement `cue mod tidy`.
  * Imlement initial version of the GitHub app.
* Phase 2:
  * Implement GitHub auth-based registry accounts for publishers.
  * Implement `cue mod publish`.
  * Retain the GitHub app for those people who want Go-style "auto publish".

As discussed earlier, later phases will add support for private modules amongst
other things.

We look forward to sharing updates to this plan in due course.


