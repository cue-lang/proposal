# Eliminate Embedding-Based Struct Opening Semantics

2025/8/19

**Status:** Draft
**Lifecycle:** Proposed
**Authors:** mpvl@
**Relevant Links:**
  - [#3757](https://github.com/cue-lang/cue/issues/3757) - eval: rethink closedness
  - [#2023](https://github.com/cue-lang/cue/issues/2023) - evaluator: embedded definition cannot be opened recursively
  - [#1576](https://github.com/cue-lang/cue/issues/1576) - Does embedding recursively open definitions?
  - [#3638](https://github.com/cue-lang/cue/issues/3638) - "incomplete value" regression in evalv3 which appears to be order dependent
**Reviewers:** TBD
**Approvers:** TBD
**Discussion Channel** GitHub: https://github.com/cue-lang/cue/discussions/4032

## Objective / Abstract

This proposal eliminates the implicit opening of closed structs through
embedding and introduces an explicit postfix `...` operator for
recursively opening composite structs.
This change simplifies CUE's semantics, reduces user confusion, and enables
clearer expression of type extensibility patterns.

## Background

CUE currently uses the "embedding position" within a struct to recursively open
up definitions.
This mechanism allows schemas to be extended without causing "closedness"
errors:

```cue
#MetaData: {
    kind: string
    name: string
}

#MyObject: {
    #MetaData

    special: string
}
```

If instead we used regular unification:
```cue
#MyObject: #MetaData
#MyObject: special: string
```
CUE would reject `special` as not allowed by `#MetaData`.

Embedding also provides an unambiguous way to specify field order,
which is important in some contexts.

However, this distinction between embedding and unification positions creates
significant confusion.
Consider for example this disjunction:

```cue
a1: #A  // fields of #A are enforced here

a2: { if true { #A } } // #A is embedded here, fields are not enforced
```
The user may think `a1` and `a2` are equivalent, but they are not:
comprehensions are in the embedding position, and thus `#A` is treated as open.

This confusion is particularly problematic when unifying disjunctions, where
results can differ dramatically based on whether structs are opened.


### Non-goals

This proposal does not aim to:
- Remove the concept of closed structs or definitions
- Change how field ordering works
- Modify the behavior of pattern constraints
- Alter the fundamental unification semantics beyond struct opening

## Overview / Proposal

We propose removing the use of embedding position for determining struct
closedness and instead introducing an explicit postfix `...` operator to
recursively open composite structs.

Under this proposal:
1. Embedding position only influences implicit field order
2. The `...` operator explicitly opens closed structs for extension
3. No implicit rules about embedding closed structs affecting outer closedness
4. Other than field order, embedding a value is exactly equivalent to unifying with it

## Detailed Design

### The Postfix `...` Operator

The new postfix `...` operator recursively opens a composite struct, making it
accept additional fields beyond those defined in the original schema:

```cue
#A: {
    field1: string
    field2: int
}

// Opens #A to allow additional fields
openA: #A...

// Can now add fields without error
openA: extraField: bool
```

### Semantic Changes

#### Before (Current Behavior)
```cue
a: {
    #A
    foo: string
}
```
Even though `a` is not a definition, using `#A` in embedding position implies
that `a` is closed.
This requires complex accounting in the implementation.

#### After (Proposed Behavior)
```cue
a: {
    #A
    foo: string
}
```
`#A` is treated as a regular closed struct and will reject `foo` if it's not
part of `#A`.

To allow extension of `#A`:
```cue
a: {
    #A...
    foo: string
}
```
Key difference: `a` is now open.
`a & {bar: 1}` would be allowed even if `bar` is not part of `#A`.

This change aligns with CUE's principle that `a: X` should mean the same as
`a: { X }`.
Since `#X...` now means a value is open, even for `a: #X...`, having it within
as struct should now no longer close this struct.
As it happens, this also considerably simplifies the implementation of
closedness.

#### Openness Persists Across References

Once a `...` is applied, a struct remains open even when referenced elsewhere:
```
#A: {foo: int}

let X = #A...
a: X
a: bar: 1 // allowed

b: #B...
c: b
c: bar: 1 // allowed
```

#### Issue 2023

cuelang.org/issue/2023 refers to the inability to recursively open up `#E`
in the following example.

```
#D: x: int
#E: d: #D

#F: {
	#E
	...
}

x: #F
x: d: x: 9
x: d: y: 10 // not allowed
```

Note that with the new semantics,
```
#F: {
	#E...
	...
}
```
this behavior would not change: the fact that `#F` closes things recursively
still holds, and we would still need to add something like `d: {...}` to
allow another field within `d`.
A benefit of this staying the same is, of course, that a transition will
be less disruptive for existing code.
But we may want to consider other mechanisms if something like this is really
wanted.

That said, we do now have an easier way to circumvent the issue on the
"call side":
```
x: #F...
x: d: x: 9
x: d: y: 10 // allowed
```


### New Use Cases Enabled

The `X...` notation enables defining open types for all specializations of a
type:

```cue
#Mammal: {
    species: string
    warmBlooded: true
}

#Dog: {
    #Mammal
    breed: string
}

#Cat: {
    #Mammal
    lives: int | *9
}

// Before: enumerate all mammals
mammal: #Dog | #Cat

// After: accept any extension of #Mammal
mammal: #Mammal...
```

Note that `#Mammal...` accepts any CUE value conforming to the `#Mammal`
schema, including values that may not explicitly extend `#Mammal` but have the
required fields.

### Implementation Simplifications

The proposal significantly simplifies the implementation by:

1. **Eliminating special-case handling**: No need to track whether a struct is
   used in embedding position
2. **Simplifying unification**: All struct unifications follow the same rules
   regardless of position
3. **Reducing evaluation complexity**: No need to retroactively close structs
   based on embedded content

### Migration Path

This is a breaking change with semantic difference that can be subtle.
So we need to be careful about its introduction.

Also, we will have different modules with different language versions.
The two semantics will have to be able to coexist for a considerable
length of time.

Luckily, we can have these implementations coexist. Files using the newer
semantics also benefit from the improved implementation incrementally.

1. **Introduce per-file experiments**: We will use CUE's per-file
    `@experiment(explicitopen)` attribute to allow users to enable this feature
    on a per-file basis. We will do so for one or two language versions.
    After that, using a higher language version will force-enable the feature.
1. **cue fix**: Provide a `cue fix` to transition old files.
1. **Warning**: detect cases that will break after a transition and warn
   users of this. We may do this as part of `cue fix --check` or some other
   tooling.
1. **turn of old behavior**: Remove old behavior for later language versions

Note that the old behavior will _still_ be supported for a considerable
length of time, as long as the respective modules use older language versions.
We hope that the addition of new features will ultimately lead to more
upgrades and gradual adoption.
We could consider fully deprecating the old behavior with a major update to
CUE v1.0.

#### `cue fix`

The fix would be applied to any file for which the new behavior is not yet
default, which supports the experiment, and for which the experiment is not
yet enabled (using `@experiment(explicitopen)`).

A `cue fix` could be written to automatically add `...` to any embedding
position that relies on the old behavior.
Furthermore, if a closed struct was embedded within a non-definition, we
introduce a definition to close it:

```
// before
a: {
    #A
    foo: string
}

// after
a: _#a
_#a: {
    #A...
    foo: string
}

// Alternative: Use a closeAll function instead of temporary fields
// This could avoid potentially awkward source translations
a: closeAll({
    #A...
    foo: string
})
```
This indirection can be omitted if `a` is definition itself:
```
#a: { // no need to close again, as #a already does so.
    #A...
    foo: string
}
```

By adding the `@experiment(explicitopen)` annotation to the file, we can mark
the transition complete.

Note that at any point in the the transition it is well defined whether
`a: { #A }` is interpreted the old way or new way, even for different language
versions.

## Alternatives Considered

### Keep Current Behavior with Better Documentation

We could maintain the current embedding semantics but improve documentation and
error messages.

**Pros:**
- No breaking changes
- Existing code continues to work

**Cons:**
- Confusion persists
- Implementation remains complex
- Violates principle of least surprise

### Use builtin

We considered alternative syntaxes like `open(#A)``.

**Pros:**
- More explicit about the operation
- No extra syntax
- Could potentially carry more semantic information

**Cons:**
- Less intuitive than postfix `...`
- Doesn't align with existing `...` pattern for "more of"
- More verbose
- non-functional behavior of something that looks like a function

## Cross-cutting Concerns

### Backwards Compatibility

This is a breaking change that will require migration.
The impact assessment shows:
- Most code using embedding for composition will need updates
- Code using embedding only for field ordering is unaffected
- Tooling can automate most migrations

### Performance

The simplified implementation should improve performance:
- Reduced complexity in struct evaluation
- Fewer special cases to handle
- More predictable memory usage patterns

### Tooling

All CUE tooling will need updates:
- Parser: Support for postfix `...` operator
- Evaluator: New opening semantics
- Formatter: Proper handling of `...` syntax
- Language server: Updated completion and diagnostics

### Documentation

Comprehensive documentation updates needed:
- Language specification
- Tutorials on struct composition
- Migration guides
- Updated examples throughout documentation