


This proposal replaces https://github.com/cue-lang/cue/issues/822.
Thanks to all of the comments and suggestions contributed there.
The proposal presented here is considerably simpler and changes less
about the current semantics.

# Objective
We introduce a new field type, a _required_ field, with the aim to address
various shortcomings of CUE while at the same time allowing for simpler semantics for other features we wish to introduce in CUE.

# Background
One of CUE’s strengths is that it allows specifying types, constraints,
and values within a unified framework. This means that fields in CUE have many different roles.
For example, they can define simple JSON-like data, add derived fields that
compute fields from these data fields (“templatize”), add constraints to data, or define top-level schema types (also constraints).

## Output and interpretation modes
There are two major modes for outputting and interpreting CUE: 
- schema: produce a schema with all constraints intact
- data: produce only concrete data after validating all constraints

The schema mode corresponds to the `cue def` command.
The data mode corresponds to `cue export`. `cue eval` lives at a poorly specified point in between the two
and should probably be deprecated or could at least benefit from some clarifications.
This is covered in https://github.com/cue-lang/cue/issues/337.

### Schema mode
The main goals of schema-mode evaluation are:

1. Simplification of schema (after unifying two schemas, for instance)
1. Validate that a schema, or the unification of two schemas, is internally consistent; that is, there are no errors.

Schema mode preserves all constraints: generating CUE in schema mode requires all fields,
including optional ones, to be produced. 

### Data mode
In data mode, all [regular fields](https://cuelang.org/docs/references/spec/#definitions-and-hidden-fields)
must be concrete, recursively, and are output.
It is an error for values to be non-concrete, like `a: string`,
or yield an incomplete error, like `a: b + c`, where `b` or `c` are not concrete.

### Validation
Validation can occur in either mode.

`cue vet` is used for both modes of interpretation.
By default it uses schema mode, but it can be explicitly set to a data mode by using the `-c` flag.

## Field Semantics
We identified the following list of common interpretation or use cases for field semantics:

1. A field is defined with the given value, which must evaluate without error (regular field)
1. A field _may_ be defined, in which case the associated constraint applies (optional field).
1. A field _must_ be defined elsewhere and satisfy an associated constraint (required field).
1. A field must not exist (optional field constraint where the constraint is an error value).
1. A field is only defined if some condition is met (conditional field).

In effect there are two types of fields: those that provide constraints and those that populate values. Below we discuss these use cases within particular usage contexts.

### Data and export
In CUE, we classify data as [regular fields](https://cuelang.org/docs/references/spec/#definitions-and-hidden-fields)
(so no definitions or hidden fields), with only concrete values.
Any JSON value is data in CUE.

In contrast to JSON, but like YAML, CUE permits fields with top-level references to other data fields
to be classified as data.

### Schema and constraints
There are two common cases for defining fields in schema:

1. If the user specifies a field `x`, apply constraint `c`.
1. The user _must_ specify a field `x` with constraint `c`.

Orthogonally to this, the user may wish to specify that `c` must be concrete.
This is not done explicitly, but rather is enforced implicitly in data mode
by requiring that any regular field (so not a constraint `a?: x`) must be concrete.

Case 1 can be expressed as:
```
x?: c
```
Case 2 is currently expressed as:
```
x: c
```
The requirement that the user must specify a field is implicit.
This works as desired when `c` is not concrete: because “non-optional” fields are required to be concrete.
It will force the user to specify the field with the concrete value.

It is not possible, however, to force a user to specify a field with a specific concrete value,
which includes any list or struct.
This is because there is no syntactic distinction between a field that is a constraint,
but required, and a regular field: the required property is implied by the fact that regular
fields must be concrete upon output.
That is, a regular field containing a concrete value that must be specified is
indistinguishable from a regular field with a concrete value specified by the user.

See, for instance, https://github.com/cue-lang/cue/issues/740 where this is problematic.

### Derived fields and templating
The purpose of templating, including deriving fields, is to automatically populate fields for a user.
Possible use cases are:

* providing a set of defaults, 
* defining a higher level of abstraction that is mapped to some more concrete representation
* converting between two versions of an API.

Examples of such fields are:

1. Defaults: `a: *1 | int`
1. Derived fields: `a: b + c`
1. Derived constraints: `a: b + c`, but only if `b + c` is defined.

All derived fields use regular, non-optional, fields.
If a field should only be added conditionally, one currently has to revert to rather verbose methods, such as
```
a?: int
b?: int
if a != _|_ && b != _|_ {
  c: a + b
}
```
Case 3 is common (see https://github.com/cue-lang/cue/issues/1232).
Having something much more compact would be nice: we have some thoughts for a possible proposal to address this.

### Policy
Policy is like schema but tends to have more derived constraints.
A key difference with schema is that policy is often not defined by the team or organization
that defined the original schema.
A policy typically just adds constraints to an existing schema.
An example use case typical to policy is making a field required that was previously not required.
Another is making constraints more specific, without making them concrete or implicitly defined.

```
#Def: {
    name?: string
    speed?: number
}
#DefPolicy: #Def & {
    name?:  strings.MinRunes(1)
    speed?: >100
}
```
# Proposal
CUE currently has the “optional field constraint”, denoted `foo?: value`.

We propose adding a “required field constraint”, denoted `foo!: value`, which is like `foo?: value`,
but requires a regular value be provided for `foo` (a field `foo:` without  `!:` or `?:`).

We refer to optional field constraints and required field constraints collectively as “field constraints”.

As a general rule, we consider that all data and data templating should be defined with regular fields,
whereas schemas would be defined in terms of field constraints.
Of course, CUE being CUE, mixing these two fields is allowed:
this rule is not a restriction but suggested as a matter of style and proper usage.

## Semantics
Informally, requiredness is a property of a field that is unified independently of a field value.

The following holds:
```
{a?: x} > {a!: x} > {a: x}
```
for any `x`, such as `int` or `1`.

For instance:
```
{foo!: 3} & {foo: 3}  	→  {foo: 3}
{foo!: 3} & {foo: int}   	→  {foo: 3} // X
{foo!: 3} & {foo: <=4}	   	→  {foo: 3}  // X

{foo!: int} & {foo: int} 	  	→  {foo: int}
{foo!: int} & {foo?: <1}   	→  {foo!: <1}
{foo!: int} & {foo: <=3}   	→  {foo: <=3}
{foo!: int} & {foo: 3}  		→  {foo: 3}
```
Note the subtle, but important difference between this proposal and #822: `foo!: value`
only requires that a field be specified; it does not require it to be concrete.
However as we are no longer proposing a change to the semantics of `x: string`, the net effect is roughly the same.

As a consequence, though, it is now possible to create a concrete value without
having the user actually provide it (see the line marked `X` in the above example).
So this loses the strictness of the original proposal.
In practice, this likely is not much of an impediment: JSON and YAML cannot specify such fields,
and thus this would not limit the ability to validate such files: they still need to specify
the actual concrete value to pass.

That completes the proposal.

## Examples
Here are some examples of how exclamation marks can be used to express things that were previously not possible.
```
#Def: {
    kind!:   "def"
    intList!: [...int]
}
```
Using required fields can also result in better error messages. Consider this schema
```
#Person: {
    name: string
    age?:     int
}
```
In the new proposal, this would be non-idiomatic, because as a general rule schema should only
be defined in terms of field constraints.

Now consider this usage of `#Person`:
```
jack: #Person & {  age:  3  }
```
In data mode the error message here is currently "jack.name: incomplete value string",
which doesn't provide much actionable information to the user to help them fix the problem.

Now consider how `#Person` would look under the new proposal,
idiomatically only using field constraints:
```
#Person: {
    name!: string
    age?: int
}

jack: #Person & {  age:  3  }
```
Now the error message would be along the lines of “jack.name: required but not defined”,
which more closely reflects the underlying problem..

This error could be resolved by adding `jack: name: "Jack"`.

# Discussion
This proposal sticks to the original semantics far more than the original proposal and
is almost completely backwards compatible with the current semantics.
We will discuss some of the consequences here.

## `a: string` vs `a!: string`
In the proposal, all types of constraints that one could specify on fields are now explicitly
defined and marked as field constraints.
This makes it visually easier to distinguish which fields are intended to be part
of the output and which are part of a schema.

Note that in this proposal, `x: string` and `x!: string` have almost identical semantics.
The latter, though, conveys more explicitly the intent that a field must be specified by the user,
and will potentially result in better error messages.

As a general rule, definitions that describe schema should replace `x: string` with `x!: string`.
The similarity in semantics should make a transition fairly straightforward.

Down the road, using `a!: string` consistently will be more important with the proposed query extensions,
as discussed next.

## Interaction with query proposal
One remaining question for the [query proposal](https://github.com/cue-lang/cue/issues/165)
is whether value filters, as opposed to field name filters, should use unification or subsumption.
Subsumption seems more natural, but has some properties that will put some awkward limitations
on any implementation.
With the semantics of `!` as defined here, it may be possible to use unification.
For instance, consider we want to filter fields in `a` based on their value.
The following notation could be used for this:

```
a: [[string]: {name!: "foo"}]: C
```
Here, we apply constraint `C` to all fields in `a` where the value has a field `name`
with a value `”foo”`.
Without the `!`, unification would simply pass for any value without a defined `name` field.
Not very useful.

For this to work as expected, it is important for schema to adhere
to the style of only using field constraints.
Suppose `a` was defined as `a: [string]: #Person`.
In this case, it is important that the `name` field in person was defined as
a field constraint (using `?` or `!`).
If, instead, it were defined as `name: string`,
all fields that did not specify a name would match the above filter.
This property may also have benefits: omitting the `!` one could,
for instance, filter all “policies” that would accept a certain name.
A proper adherence to the guideline of using field constraints for schema only,
would generally cause this to work as expected.

Whether a query implementation uses unification or subsumption remains an open question,
but the semantics of `!` might open up a straightforward way to implement
value querying without the complexities introduced by using subsumption.

## Better “OneOf” semantics
OneOf fields in protobufs are currently represented as
```
#Foo: {
    {} | { a?: int } | { b?: int }
}
```
In the old required fields proposal, it was observed that using a builtin
like `numexist`(as proposed in #943) results in better semantics and ergonomics.
For completeness, it would look like this:
```
#Foo: {
    numexist(<=1, a, b)
    a?: int
    b?: int
}
```
The resulting form will be akin to the
[“structural form”](https://kubernetes.io/blog/2019/06/20/crd-structural-schema/) for OpenAPI.

We observed that in the general form, this only really works well
if the fields passed to `numexist` are “optional fields” (field constraints):
regular fields of structs are by definition concrete and will always exist.
Sticking to defining all schema fields as constraints makes it natural and straightforward to use `numexist`.

## Encouraging the extraction of defaults pattern
As a general rule, we recommend that default values are specified independent of a schema.
This pattern ensures maximal reusability of schema and forces one to think more clearly about whether
a default is meant as a policy, a constraint or policy, or an actual baked in default.

Along with the proposed changes, we suggest that as a general rule schema only be specified
in terms of field constraints (`foo?` or `foo!`).
Default values mean something different in field constraints and are generally not very useful there.
So as a consequence of the rule that schema only contain field constraints,
the user is naturally also discouraged to include default values in schema.

To support hoisting default values from schema,
we could consider having some shorthand for the common pattern `a: *2 | _`, for instance, `a: default(2)`.
This could be considered in a separate proposal.

## Now we still have many `?` annotations
A goal of the original required fields proposal was to get rid of the proliferation of question marks (`?`).
This proposal explicitly drops this goal.
Instead, this proposal advocates being very explicit about marking the intent of a field.
That is, all constraint fields – fields of a schema – now are adorned with a marker in the proposal.

As it turned out, it was impossible to get rid of many uses of `?`.
So at the expense of more typing, there is now a clear story as to when to use field constraints
(for schema) and when not.

The objection that the existence of `?` encourages bad APIs has partially been averted,
as it is now suggested that _all_ fields of a schema be defined as field constraints.
This eliminates the issue of required fields being shorter to type and thus encouraging their use.

Furthermore, the proliferation of `?` could be mitigated in the future by allowing some shorthand annotation,
for instance:
```
#Def: ?{
    kind!:  string
    name: string
}
```
where the `?{` indicates that all unmarked fields are implicitly marked with `?`.

We consider it unlikely we will take this approach, though.
We rather favor a validation approach, like a vet rule, or an automatic rewrite, 
that ensures consistent use of `?`.


