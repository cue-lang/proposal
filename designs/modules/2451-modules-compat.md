# Proposal: CUE modules backwards compatibility

Status: **Draft**

Lifecycle:  **Proposed**

Author(s): mpvl@cue.works

Reviewers: rogpeppe@cue.works myitcv@cue.works

Discussion channel: [GitHub](https://github.com/cue-lang/cue/discussions/2451)

# Abstract

The CUE modules proposal relies on MVS.
MVS, in turn, relies on the notion of backwards compatibility.
This document describes the backwards compatibility rules for CUE modules.


# Background

In Semver, a major release is a release that is not backwards compatible.
A minor release is a release that is backwards compatible and adds
functionality.
A patch release is just fixes bugs, but does not change the API.

As CUE allows defining schema, data, validation, and templating, among other
things, it may be unclear what backwards compatibility means.
In general, it is hard to define backwards compatibility on anything
other than schema.

# Proposal

We propose to only support backwards compatibility checks for
definitions (fields starting with a `#`), based on subsumption:

- a major change occurs when a newer defintion does not subsume an older
  version,
- a minor change occurs when a newer definition subsumes an older version,
  and is not semantically equivalent to this older version,
- a patch change occurs when a newer definition is semantically
  equivalent to this older version.

We will discuss what this means in more detail below.

The enforcement of the compatibility rules is optional and merely serves
as a guide to the user.
For instance, it may be desirable to not bump a major version when
a backwards compatible change is made to change some bug, especially
when this pertains to a security issue.


# Details

## Compatibility check

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
lattice.
Moreover, it seems that it is generally more appropriate to enforce
rules for proper usage of templates, policy, etc. on the client side.


## Command line override

There are several spots in the overall module proposal where checks are
performed that we may want to be able to ignore on failed.
The CUE command line should probably support a single flag,
like `--override`, or `--ignore`,
that allows specific checks to be ignored.

We could potentially also require or allow the user to give a reason for the
override, for instance indicating the override fixes a security issue.


# Discussion

## Module author responsibilities

Module authors need to be aware of the backwards compatibility rules,
as these may influence design decisions.

As a general rule of thumb, pure schema (no defaults, no regular fields) should
be exported as `#Foo` while, for instance, an equivalent schema with recommended
default values can be provided as `Foo: #Foo & { // defaults }`.
Organizing schema this way allows module authors to change default values
without breaking compatibility rules.

Nothing stops module authors, of course, from providing templates as
definitions, thereby guaranteeing that default values will not change.
For instance, consider the following enumeration

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

## Module consumers' responsibilities

Compatibility guarantees only prevent breakage when upgrading up to a certain
point.
Module consumers should remain cautious when using schema for which they do not
control changes.
Consider this example

```go
import "example.com/third/party"

Foo: {
	party.#Schema

	newField: int
}
```

where `newField` is not defined in `party.#Schema`.
Under the compatibility rules, newer versions of `party.#Schema`
may add a `newField?: string`.
This would break the author of `Foo`.

Another example:

```go
import "example.com/third/party"

Foo: party.#Schema
Foo: existingField: _
```

Assume `party.#Schema` defines `existingField?: 5`.
`Foo` is valid and exportable, as `existingField`
will result in a concrete value.
However, at a later stage, the module publisher
could change this to `existingField?: 5 | 6`, a valid change.
This would now break the user’s configuration as exporting it
may now result in an incomplete error.

Similarly, users should also be cognizant when using templates:

```go
import "example.com/third/party"

Foo: party.IntValue
Foo: 6
```

Given a `party.IntValue` of `6`, this will pass.
However, the module publisher is allowed to change this under
the compatibility rules (note it is `IntValue`, not `#IntValue`).
Changing it to `7`, for instance, would break this user.

In all these breakage scenarios, we consider it the responsibility of the user
of a module to avoid or deal with breakage.
We envision having usage guidelines as well as tooling support
to avoid such pitfalls.


## Exceptions to the compatibility check

Under some circumstances it may be desirable to publish a newer version of a
module that is not backwards compatible.
Consider the following schema:

```go
// myserver.cue
#Request: {
	gauge: >=0 & <=1
}
```

This schema allows the value of `1` for `gauge`.
However, suppose the author inadvertently made a mistake and the server
does not actually support a `gauge` value of `1`.
A request is guaranteed to fail for such values.

A fix is made accordingly:

```go
// myserver.cue
#Request: {
	gauge: >=0 & <1
}
```

Pushing this using the same major version is a violation of the compatibility
rules.
However, not doing so may result in uncaught bugs and may even pose a
security issue.

The CUE tooling should therefore allow for a  process by which the compatibility
rule can be bypassed.

We are aware of this need and are still figuring out how to best go about this.

## Comparison to wire compatibility

Note that the CUE backwards compatibility rules do not ensure wire
compatibility.
Most notably, the rules allow newer versions to turn required fields
into optional.
Using a newer CUE schema to validate a message designated
for an older server could therefore possibly not catch an erroneous omission.

There are several workarounds for this we could consider in the future:

- allow module publishers to opt into stricter compatibility rules;
- provide “virtual” computed schemas that reflect best practices for a range of
  versions;
- validation rules that are aware of the differences with older versions.


## Incremental backwards compatibility

SemVer allows backwards incompatible changes in `v0` mode to allow for an API to
evolve until it is stable and can be moved to `v1`.
One lesson learned from Go modules is that it is often onerous
to move to `v1` as there are always uncertain aspects of the API.
As a result, many modules remain in `v0` indefinitely.
Additionally, the same flexibility does not exist for `v2`, even
though this may be desirable.

To address this issue, we are contemplating supporting annotations
in CUE using the `@api` attribute.
This would allow different parts of the API to be marked as a
different state of maturity, providing backwards compatibility checking for
stable parts while still allowing experimental parts of the API to coexist.

While this is outside the scope of this design document, we believe it is an
important consideration for an ecosystem that relies on backwards compatibility
guarantees.

