# Abstract

We provide a high-level overview of a module system for CUE.
Some of the details are to be worked out in separate subproposals.


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

The registry will be implemented as a web service using the standard
OCI protocol. Users will typically interface with this web service
through the `cue` command line.

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
- A way for the `cue` tool to publish modules to a registry.
- Use of Semantic Versioning for modules.
- Standardized module identifiers for major versions.

Each of these design aspects is motivated and explained in more detail in the
next section.

## Subproposals

Module support is a large addition to the CUE ecosystem.
Development will commence along the lines of several subproposals.
Not all of these will be completed before the first release of the registry.

The following subproposals have been made or are currently planned:


### Storage Model

The overarching design of the registry is agnostic to the storage model.
We propose to store CUE modules in OCI registries.
For applications that require local replication, this allows
building on existing tools and infrastructure.

This is proposed [here](./modules/2449-modules-storage-model.md).


### Supply Chain Security

This sub-proposal discusses security aspects relating to modules.

This is proposed [here](./modules/2450-supply-chain-security.md).


### Backwards Compatibility

We propose that the semantic versions of modules follow some guidelines
of what we consider major, minor, or patch changes.
By default, tooling will fail when proving a tag for a module that
does not follow these guidelines, but will allow users to override
such checks, as they are not always desirable.

This is proposed [here](./modules/2451-modules-compat.md).


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

The `cue.mod/module.cue` file indicates a default version of a module which
will be chosen when an import path does not contain a major version,
making import paths unambiguous when considered together with that file.

For a newly added module, the tooling will choose the latest major version that
has a non-prerelease version.

## The `cue.mod/module.cue` file

Module dependencies are configured in this file. It has the format below. Note
that there is no package clause. This is deliberate. All module-related data
must be specified in this single file. Authors should treat this schema as
closed. A field for user-defined data may be added later.

Initially we will require this to be a data-only file.

```go
// This schema constrains a module.cue file. This form constrains module.cue files
// outside of the main module. For the module.cue file in a main module, the schema
// is less restrictive, because wherever #SemVer is used, a less specific version may be
// used instead, which will be rewritten to the canonical form by the cue command tooling.

// module indicates the module's path.
module!: #Module

// The language version indicates minimum version of CUE required
// to evaluate the code in this module. When a later version of CUE
// is evaluating code in this module, this will be used to
// choose version-specific behavior. If an earlier version of CUE
// is used, an error will be given.
language?: version?: #SemVer

// description describes the purpose of this module.
description?: string

 deps holds dependency information for modules, keyed by module path.
//
// An entry in the deps struct may be marked with the @indirect()
// attribute to signify that it is not directly used by the current
// module. This will be added by the cue mod tidy tooling.
deps?: [#Module]: {
	// v indicates the minimum required version of the module.
	// This can be null if the version is unknown and the module
	// entry is only present to be replaced.
	v!: #SemVer | null

	// default indicates this module is used as a default for
	// import paths within the module that do not contain
	// a major version. Imports must specify the exact major version for a
	// module path if there is more than one major version for that
	// path and default is not set for exactly one of them.
	default?: bool
}

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


## `cue` command extensions

Here we discuss the initial set of supported commands. The first to be
implemented are `init`, `tidy`, and `publish`. Others will follow later.


### main module


### module cache

We will maintain a module cache in
[`os.UserCacheDir()/cuelang`](https://pkg.go.dev/os#UserCacheDir).

Files are stored as read-only in the module cache.
This directory can be configured by setting `$CUE_CACHE_DIR`.


### version queries


### `cue mod init`

The `init` command will remain as is. The module name may now contain a major
version suffix. If a major version suffix is not supplied, an `@v0` suffix will
be added.


### `cue mod tidy`

Usage:

```
cue mod tidy [--check]
```

`cue mod tidy` ensures that the `cue.mod/module.cue` file matches the CUE code
in the module. It adds any missing module requirements necessary to build the
current module's packages and dependencies, and it removes requirements on
modules that don't provide any relevant packages. It also adds any missing
entries to `cue.sum` and removes unnecessary entries.

`cue mod tidy` works by loading all of the packages in the [main
module](#main-module) and all of the packages they import, recursively. `cue mod
tidy` acts as if all build tags are enabled, so it will mostly ignore `@if()`
attributes, even if those source files wouldn't normally be built. The one
exception is `@if(ignore)`, which will always be ignored. `cue mod tidy` will
not consider packages in the [main module](#main-module) in directories
named `testdata` or with names that start with `.` or `_`.

Once `cue mod tidy` has loaded this set of packages, it ensures that each module
that provides one or more packages has an entry in `deps` in the [main
module's](#main-module#main-module#main-module) `cue.mod/module.cue`. `cue mod
tidy` will add a requirement on the latest version of each missing module (see
[version queries](#version-queries) for the definition of
the `latest` version). `cue mod tidy` will remove dependencies for modules that
don't provide any packages in the set described above.

If the `--check` flag is set, `cue mod tidy` will check that the requirements
are correct and the language version is present, but will not update the
`module.cue` file.

### `cue mod publish`

Usage:

```
cue mod publish version
```

Example:

```
$ cue mod publish v1.1.2
```

The `cue mod publish` command uploads the [main module](#main-module) to the
registry and tags it with the given version.


## Minimum Version Selection

We plan to use [Minimum Version Selection
(MVS)](https://go.dev/ref/mod#minimal-version-selection) for resolving
dependency versions.


### Rationale

- MVS results in stable behavior when recomputing dependencies, resulting in the
  minimum set of required changes to meet new requirements. This fits well with
  the general properties desirable for configurations.
- MVS provides a clear path to support co-versioning of CUE schemas that are
  stored along code in package managers of other languages.
- Compared to a SAT solver (as required by most more general versioning
  schemes): MVS has much more predictable running times.
- We considered using an “always on tip/latest” approach.
    - With the CUE registry it is possible to analyze the impact of schema
      changes, possibly making this manageable. We recognize, though, that many
      users want something more predictable.
    - An always on HEAD approach can still be simulated with MVS.


# Implementation Plan

All the features described here have been implemented and are currently
available, guarded by the `CUE_EXPERIMENT=modules` flag.
