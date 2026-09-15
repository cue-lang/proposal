# Proposal: functions in CUE

2026/07/10

*   **Status**: Draft
*   **Lifecycle**: Proposed
*   **Author(s)**: mpvl@
*   **Relevant Links**: experimental implementation in `cue-lang/cue`,
    enabled per file with `@experiment(functions)`
*   **Discussion Channel**: https://github.com/cue-lang/cue/discussions/4484

### Objective

This proposal adds functions to CUE: function literals with typed
signatures, function types, calls with positional and labeled arguments,
and rules for combining signatures and for evolving them compatibly.

Functions build on CUE's existing lattice-based model. A call can be
understood as constructing a fresh struct of parameter values, unifying
the arguments with their constraints, and selecting a result constrained
by the body and result type. Optionality reuses field semantics, and a
parameter default is declared with `=` and consumed at the call.
Signature composition also checks the function's calling contract;
backwards compatibility compares accepted calls and result constraints,
using subsumption in the appropriate direction for each. The struct
model explains evaluation under the hood. The detailed rules below
specify the behavior; the illustrative hidden fields are not surface
syntax or a required evaluator representation.

Goals:

*   **Clarity**: a small number of concepts and predictable rules; users
    reason about functions with the same lattice they already use for
    everything else.
*   **Precise API signatures** for builtins, user-defined functions and,
    eventually, external services.
*   **Idiomatic, non-surprising syntax** comparable to languages like
    Python, Swift, and Go.
*   **Evolvability**: a signature can evolve with as much backwards
    compatibility as possible, with compatibility checked through argument
    binding and subsumption of the input and result constraints.
*   **Minimal implementation complexity** by leveraging CUE's existing
    semantics.

Non-goals:

*   Turing-complete recursion for CUE functions. Recursion is rejected
    (see [Recursion](#recursion)).
*   Dependent parameter types, such as a parameter constraint that refers
    to another parameter. The syntax is reserved so this can be added
    later (see [Scope of names in a signature](#scope-of-names-in-a-signature)).
*   Native variadic parameters. A variable number of arguments is passed
    as a list (see [Variadic arguments](#variadic-arguments)).
*   Foreign function interfaces (FFI), purity, and side effects. These
    are important motivations for this design and are sketched under
    [Future extensions](#future-extensions), but their semantics are
    deferred to a follow-up proposal.
*   Validator semantics. CUE internally treats validators like
    `strings.MinRunes(3)` as partially applied functions; cleaning up
    their specification is a separate follow-up.

### Background

CUE is a lattice-based constraint language: every value inhabits a
partially ordered set, and the fundamental operation is unification
(`&`). Structs already provide labeled fields, optional and required
markers, closedness, and default values. In fact, many users already use
structs to simulate functions:

```cue
sum: {
    a:   int
    b:   int
    out: a + b
}

result: (sum & {a: 1, b: 2}).out
```

This pattern works, but it has real drawbacks: there is no positional
calling, the "call" is verbose, the parameters and the result are not
distinguished from ordinary fields, there is no arity checking, and
error messages point at struct internals rather than at a call.

At the same time, CUE already has functions: the builtins of the
standard library. Their signatures, however, cannot be expressed in the
language itself. This makes it impossible to precisely document
builtins, to type a parameter of a higher-order function, or to validate
that a user-supplied function conforms to an expected interface.

This proposal extends CUE with function values as a natural extension of
the existing model. Parameter lists are a restricted form of struct
declarations, and each call gives those declarations argument values.
An omitted optional parameter remains absent; a declared default can
supply an otherwise unbound parameter. The call's activation behaves
like a struct, while the function retains its implementation identity,
calling contract, and unevaluated expressions separately. Signature
composition reuses CUE constraints and adds rules for matching parameters.
Compatibility reuses subsumption to compare the constraints after checking
that existing argument bindings remain valid.

### Overview

This section describes what a user needs to know to write and call
functions. The subsequent sections define the details.

A function is written as a literal: parameters in parentheses, an
optional result constraint after `->`, and the body after a final `:`.

```cue
@experiment(functions)

Sum: func(a: int, b: int) -> int: a + b

p: Sum(1, 2)       // 3, by position
l: Sum(a: 1, b: 2) // 3, or by name
m: Sum(1, b: 2)    // 3, or mixed; positional arguments come first
```

Parameters take five forms, on a gradient from purely positional to
name-only:

```cue
a: int      // named: callable by position or by the label a
a!: int     // required: must be supplied, by label only
a?: int     // optional: may be omitted, supplied by label only
_~x: int    // positional-only: bound by position, named x in the body
int         // anonymous: bound by position, not referable from the body
```

A bare `a: int` is callable both ways, like in Python, Go, or Ada. Its
name and its position are then both part of the function's contract:
renaming or reordering it breaks callers. The other forms are the
opt-outs. Marking a parameter `a!:` (required) or `a?:` (optional) makes
it name-only and order-independent, which suits optional,
self-documenting arguments. For a parameter that should have a name in
the body but no name in the contract, alias an anonymous positional:
`_~x: int` is supplied by position and referenced as `x` in the body,
and `x` can be renamed freely without affecting any caller.

A parameter may declare a default after its constraint, written with `=`.
A call may omit such a parameter, and the default is then used as the
argument. A supplied argument, whatever it is, makes the default
irrelevant, so an argument that carries its own default keeps it:

```cue
@experiment(functions)

add: func(a: int, b: int = 10) -> int: a + b
x:   int | *5

d: add(1)    // 11
s: add(1, x) // 6: x's own default applies, not b's
```

A literal without a body denotes a *function type*. Unifying a function
type with a function value tightens the value: the type's constraints
are enforced on every call. This is how a schema can require a field to
hold a function of a particular shape, including a builtin:

```cue
@experiment(functions)

T: func(a: int, ...) -> number // open: admits further parameters
f: T & (func(a: int, b: int) -> int: a + b)
x: f(1, 2) // 3
```

A call whose argument list ends in `...` is a partial application: it
binds the given arguments and yields a function over the remaining
parameters.

```cue
@experiment(functions)

add: func(a: int, b: int, c: int) -> int: a + b + c
f:   add(1, ...) // a function of b and c
x:   f(2, 3)     // 6
```

Two properties are worth knowing from the start:

*   **Recursion is an error.** A recursive call, direct or mutual, does
    not terminate in finite structure and is reported as a structural
    cycle. Calls may otherwise be nested and repeated freely, including
    nested calls to the same function. CUE remains non-Turing-complete.
*   **Functions are opaque values.** A function value unifies with
    itself and with function types; unifying it with any other value,
    including a different function value, results in `_|_`.

The feature is gated per file by the `@experiment(functions)` attribute.
A file without the attribute parses exactly as before.

### Detailed Design

A note on terminology, aligned with the existing specification: a
*parameter* is a formal name and constraint declared in a signature; an
*argument* is an actual value supplied in a call.

#### Syntax

The following keyword introduces a function literal, and a new `->`
token separates the parameter list from the result constraint:

```ebnf
FuncLit    = ClosedFunc | OpenFunc .
ClosedFunc = "func" "(" [ ParamList [ "," ] ] ")" [ "->" Expression ] [ ":" Expression ] .
OpenFunc   = "func" "(" [ ParamList "," ] "..." [ "," ] ")" [ "->" Expression ] .
ParamList  = ParamDecl { "," ParamDecl } .
ParamDecl  = [ identifier [ "~" identifier ] [ "!" | "?" ] ":" ] Expression [ "=" Expression ] { attribute } .
```

The first expression of a parameter is its constraint; the expression
after `=`, if any, is its default. One production covers every parameter
form: a parameter is one declaration shape with or without a label.

The call syntax is extended to allow labeled arguments:

```ebnf
Argument  = [ identifier ":" ] Expression .
Arguments = "(" [ Argument { "," Argument } [ "," ] ] [ "..." ] ")" .
```

A parameter declaration is a deliberately restricted form of a struct
declaration. The grammar is defined explicitly rather than as a general
declaration with exclusions, so that what is not a parameter is not
accidentally admitted:

*   let clauses, attribute declarations, comprehensions, and pattern
    constraints are not permitted;
*   a parameter's label must be a single, plain identifier: multi-part,
    string, dynamic, and definition labels (`#a`) are not permitted.
    This also guarantees that every named parameter is addressable by an
    argument label, which the call grammar restricts to identifiers;
*   the blank identifier `_` declares an anonymous positional parameter
    and may not be marked optional or required (`_?`, `_!`), as such a
    parameter could never be bound;
*   an embedded expression (`int`) also declares an anonymous positional
    parameter;
*   an ellipsis must be the final element of the list and marks a bodyless
    signature as open (see [Function types](#function-types)); an open
    signature cannot have a body;
*   the dual form of a postfix alias may not be used in a parameter;
*   an optional parameter (`a?:`) may not declare a default, since it is
    absent when a call omits it;
*   a default mark (`*`) written directly in a parameter's constraint or
    default is an error: a parameter default is declared with `=`. A mark
    nested inside a struct or list literal, or reached through a
    reference, is an ordinary part of the parameter's value.

Attributes attached to a parameter are treated like field attributes;
they do not influence evaluation. They provide a natural place for
FFI-oriented annotations in a follow-up proposal.

The grammar adds no variadic forms (see
[Variadic arguments](#variadic-arguments)).

#### Parameter forms and ordering

Whether a parameter is supplied by position or by name — and whether
its name is part of the API contract — is the most important thing a
signature commits to. The form decides it:

*   **Anonymous positional** (`int` or `_: int`): supplied by position
    only. It has no contract name and cannot be referenced from the
    body.
*   **Positional-only with an alias** (`_~x: int`): supplied by
    position only; the body refers to it as `x`. The alias is internal,
    so it can be renamed freely without breaking any caller.
*   **Named** (`a: int`): callable by name or position. The name is
    final, and because the parameter is also positionally callable, its
    position is part of the contract too.
*   **Required, name-only** (`a!: int`): must be supplied, by label
    only, order-independent. It is the name-only counterpart of a named
    parameter, so a default (`a!: int = d`) makes it omittable in the
    same way: this is the keyword-only parameter with a default that
    other languages offer, bound to `d` in the body when the call omits
    it.
*   **Optional, name-only** (`a?: int`): may be omitted; supplied by
    label only. When omitted it is absent in the body, as an absent
    optional field is, so it cannot declare a default.

Parameters follow a nameability gradient: positional-only forms come
first, then named parameters, then name-only parameters.

*   A parameter that cannot be bound by label — anonymous or
    positional-only — may not follow a parameter that is declared with
    a name. Otherwise the anonymous parameter could only be reached by
    position, which would force the named one to be supplied
    positionally as well, leaving its name unusable.
*   A parameter that can be bound by position — any parameter not
    marked `!` or `?` — may not follow a required or optional
    parameter, as its position would otherwise be ambiguous.

A parameter name may be a hidden identifier (`_a`). Hidden identifiers
are package-scoped in CUE, and this carries over to parameters: a
hidden parameter can be bound by label only from within the declaring
package; to any other package it is effectively positional-only. This
provides a middle ground between a contract name and a body-only
alias: the name is part of the package's internal interface, not of
its public API, so renaming it — or making it visible (`_a:` → `a:`),
which only adds a way to call it — does not break external callers.
Note that marking a hidden parameter required (`_a!:`) makes the
function uncallable from any other package, since only the declaring
package can write the label; a lint could flag this on exported
functions.

Parameter names must be unique within a parameter list. These are
declaration-level well-formedness rules, rejected when the signature is
compiled, not by unification:

```cue !
@experiment(functions)

bad: func(a: int, int) -> int: 1
// error: positional parameter after named parameter
```

#### Parameter defaults

A parameter default is declared after the constraint with `=`, on every
form except the optional one:

```cue
@experiment(functions)

f: func(_~n: int = 0, int = 2, a: int = 1, b!: string = "x") -> int: a
```

The rules are few, and they are deliberately not the rules of CUE's
disjunction defaults:

*   **Omission is declared, not inferred.** A call may leave a parameter
    unbound exactly when the parameter is optional or declares a
    default. What the constraint evaluates to plays no part: `p: #Port`
    with `#Port: int | *80` is a parameter that must be supplied, whose
    value then carries the default of `#Port` like any other value.
*   **An omitted parameter takes its default as the argument.** The call
    proceeds as if the default expression had been supplied, resolved in
    the scope of the signature rather than the caller's; the parameter is
    the unification of its constraint and the default.
*   **A supplied argument makes the default irrelevant.** Whenever an
    argument is bound to the parameter — by position, by label, or by an
    earlier partial application — the default plays no part in the call.
    An argument that is non-concrete (`_`, an unresolved disjunction) does
    not cause the default to apply, and an argument that carries its own
    disjunction default flows into the parameter unchanged.
*   **A default is checked at the declaration.** When a function literal is
    evaluated, each default is checked against its parameter's constraint;
    `func(a: int = "x")` is an error whether or not the function is ever
    called. A default that cannot be evaluated yet leaves the check
    undone; the call that omits the parameter reports the incompleteness,
    or the cycle for a default that calls its own function.
*   **A default is not a value.** It exists only in a signature: no
    expression denotes one, so nothing can pass, store, or reference a
    parameter default, and the body sees only the value the default
    produced.

The third rule is the reason for the design. Reusing disjunction defaults
would unify a caller's own default with the parameter's, canceling both
whenever they differ (see [Alternatives
Considered](#reusing-disjunction-defaults-for-parameters)); with a declared
default, the caller's value is exactly what the body sees.

#### Scope of names in a signature

Parameter constraints, parameter defaults, and the result constraint are
resolved in the scope in which the literal is declared, not in the body's
scope, and may refer to fields of enclosing scopes:

```cue
@experiment(functions)

limit: int | *7
f:     func(a: string, b: limit) -> int: b // b's constraint refers to limit
out:   f("shadow", int) // int | *7; a default reached through limit does not make b omittable
```

Referring to *another parameter* from a parameter constraint, a parameter
default, or the result constraint is reserved and reported as an error. Without
this rule, such a reference would silently bind to a like-named field
of an enclosing scope — surprising, and it would foreclose a possible
future dependent-parameter extension (see
[Dependent parameter types](#dependent-parameter-types)):

```cue !
@experiment(functions)

f: func(x: int, y: >x) -> int: x
// error: cannot refer to parameter "x" in a parameter constraint or return type
```

The reservation applies one literal at a time: a function literal
nested inside a constraint or result type resolves its own body
normally, so a function-typed result or parameter works, while the
nested literal's own signature reserves its own parameters.

Within the body, parameters shadow fields of enclosing scopes. A
function value captures the scope in which its literal is declared:

```cue
@experiment(functions)

base:  10
scale: func(a: int) -> int: a * base // captures base
```

#### Struct semantics

This section explains how function evaluation relates to structs. An earlier
implementation used a struct encoding; the experimental evaluator now
represents functions and calls natively. The model remains useful for
understanding parameter binding and constraint evaluation, but it is not
a complete syntactic desugaring of every function operation. In particular,
signature admission, opaque identity, and argument-presence checks have
the rules stated in their own sections.

There are two stages: retaining a function's declarations, and constructing
an activation for a particular call.

**Stored declarations.** A function value retains its implementation and
captured scope, its parameter declarations, and any attached function types.
Constraints, defaults, and the body retain their expressions and declaring
environments. They are not ordinary, already evaluated fields of a shared
struct. This distinction permits parameter and result conflicts to be
reported at a call without making the uncalled function bottom. The
declaration-time default check is a separate operation, as described under
[Parameter defaults](#parameter-defaults).

The implementation supplies a fixed sequence of positional slots and a set
of name-only parameters. Attached signatures constrain those parameters and
may supply a contract label for an unnamed positional slot. A closed type
restricts which additional parameter declarations it admits; it does not
contribute a scalar arity that must equal the implementation's. For example,
`func(a: int)` can admit an implementation with an additional defaulted
parameter `b`. The implementation still has two positional slots.

Parameter matching establishes a shared label relation before a call binds
arguments. A named positional parameter `x` at ordinal `n` can be pictured
as both `_slots[n]._label: "x"` and `_labels.x: n`. Unification then explains
the label rules: an absent label acquires `x`, repeated `x` agrees, and two
different labels at one ordinal conflict. The reverse mapping prevents one
label from naming two ordinals. Hidden labels include their package identity;
their spelling alone does not identify them. Name-only constraints follow
the same label when an attached signature exposes it on a positional slot.
An admitted type parameter that matches no implementation parameter creates
no callable slot. Admission is governed by [Function types](#function-types).

**Call activation.** Once arguments have been bound, each call gets fresh
parameter cells. Positional parameters can be pictured as `_slots[n]._value`;
name-only parameters as fields under `_named`. The body refers to these
cells. A supplied argument defines its cell, a used default defines it with
the default expression, and an omitted optional parameter contributes only
an optional field constraint. Each parameter constraint applies in its
declaring environment. The result cell unifies the body with the result
constraints.

For example, these declarations

```cue
_add1: func(_~x: int) -> int: x + 1
_add1: func(x: int) -> int
```

give either `_add1(2)` or `_add1(x: 2)` an activation modeled by:

```cue
let frame = __closeAll({
    _slots: [{
        _value: int
        _label: "x"
    }]
    _labels: x: 0
    _named:  {}
    _result: int & (_slots[0]._value + 1)
})

result: (frame & {_slots: [{_value: 2}]})._result // 3
```

Here `__closeAll` denotes CUE's existing internal recursive-closing operation,
so the generated nested structs are closed as well as the outer one. It is
explanatory notation, not new function syntax. The generated bookkeeping is
closed after parameter matching; parameter values keep the open or closed
constraints their declarations give them. A general encoding must preserve
that distinction instead of recursively closing arbitrary argument values.
The evaluator also checks labels explicitly, including hidden labels, which
ordinary struct closedness does not exclude. Generated bookkeeping names
occupy a separate namespace from user parameter names.

Argument binding precedes value unification. It records whether each
parameter was supplied and rejects duplicate, unknown, surplus, or missing
arguments. This information cannot be recovered from the unified values:
supplying `_` still counts as supplying an argument, and supplying the same
value twice is still a duplicate. In a list-based illustration, an argument
fragment must use an open tail or be padded to the implementation's slot
count; a shorter closed list would introduce an unintended length conflict.

For native CUE functions, an unbound parameter's declared default is then
used as its argument. Defaults stay in the stored declarations until this
step; they are not extra evaluated fields of the activation. Thus a supplied
argument never meets the parameter default, and an unused recursive default
does not introduce a cycle into that call. Builtins use their own omission
rules, described under [Function types](#function-types).

Partial application retains the bindings and their environments instead of
evaluating the body. Later positional arguments address the remaining
unbound positional slots. The eventual full call constructs the activation;
it neither mutates an earlier activation nor shares parameter bindings with
an independent call. Function and partial-value identity are kept separately
from this explanatory frame.

#### Calls

Given a function value `f`, the call `f(a1, a2, … an)` supplies
arguments to `f`'s parameters. An argument is positional or labeled;
positional arguments must precede labeled arguments. Positional
arguments bind the function's positional parameters in order; a labeled
argument binds through the shared label relation. Arguments are
evaluated in the scope of the caller. After binding and default completion,
parameter and result constraints are enforced as illustrated by the call
activation above.

A call results in `_|_` if it

*   binds a parameter both by position and by label,
*   labels a parameter that is unknown or not bindable by label,
*   supplies more positional arguments than there are positional
    parameters, or
*   leaves a non-optional parameter unbound without a declared default.

Any parameter may be left unbound if it is optional or declares a
default; an unbound default is used as the argument, resolved in the
signature's scope (see [Parameter defaults](#parameter-defaults)). Omission
is decided per parameter: there is no separate trailing-elision rule. A
positional-only parameter is omitted by supplying fewer positional
arguments; a named one may also be skipped while later parameters are
bound by label.

The selected result field is the unification of the body and the result
constraint, evaluated with each parameter cell constrained by its declaration
and its argument, or its default when left unbound. Because the result is
that unification, a body that produces a value outside the declared result
constraint yields `_|_`.

```cue
@experiment(functions)

sum:  func(a: int, b: int) -> int: a + b
key:  func(a!: int, b?: int) -> int: a
pick: func(a: int = 5) -> int: a
add:  func(a: int, b: int = 10) -> int: a + b
x:    int | *5

p: sum(1, 2)       // 3
l: sum(a: 1, b: 2) // 3
m: sum(1, b: 2)    // 3
k: key(a: 4)       // 4
d: pick()          // 5
o: add(1)          // 11
s: add(1, x)       // 6: x's own default applies, not b's

e1: sum(1, 2, 3)            // _|_ // too many positional arguments
e2: sum(a: 1, b: 2, c: 3)   // _|_ // unknown argument c
e3: sum(1, a: 2)            // _|_ // argument a provided by position and label
e4: key(4)                  // _|_ // missing required argument a
e5: sum(1)                  // _|_ // missing argument b
e6: (func() -> string: 1)() // _|_ // conflicting values 1 and string
```

An argument that is not yet defined makes the call incomplete, exactly
as it would any other operation consuming it. Calling an unresolved
disjunction of function values reports an incomplete error rather than
failing hard, and resolves through a default when one is present.

#### Function types

A function literal without a body denotes a function type. Calling a
function type is an error. A trailing ellipsis marks a signature as
open: an open type is a partial signature that admits parameters beyond
the ones it declares and cannot have a body. It becomes callable only with a
closed function value or builtin; without an ellipsis a signature is closed.

Unifying two function types aligns positionally callable parameters by
ordinal and name-only parameters by label, and yields a type in which
the constraints of matched parameters and the result constraints are
unified when used by a call. Matched parameters must agree on whether they
carry the `!` marker. This is a signature-matching rule, additional to
ordinary field unification: `{x!: int} & {x?: int}` is a valid struct,
whereas the corresponding function types conflict under this rule. A parameter
matched in only one of the types is an error unless the other type is
open or the unmatched parameter is optional or declares a default. The
unified type is open only if both types are open.

Defaults are part of a signature. A function type may declare parameter
defaults; a default declared by one of the unified signatures applies to
calls through the result, and defaults declared by both for the same
parameter must unify — a disagreement is an error when the signatures
are unified. Neither signature overrides the other: unification is
commutative, and deriving a variant of a function with a different
default is deliberately not supported (see [Alternatives
Considered](#deriving-a-variant-with-a-different-default)).

```cue
@experiment(functions)

T: func(a: int = 2) -> int
f: T & (func(a: int) -> int: a)
x: f() // 2, the type's default applies
e: T & (func(a: int = 3) -> int: a) // _|_ // conflicting defaults for parameter a
```

Each aligned positional slot has at most one contract label: no label plus
`x` yields `x`, `x` plus `x` yields `x`, and `x` plus a different label `y`
conflicts. The same label cannot identify two slots. Thus `func(int)` composes
with `func(x: int)`, while `func(x: int)` conflicts with `func(y: int)`.

The shared-slot model explains why an anonymous slot accepts a label
contributed by another signature, while two competing labels conflict rather
than becoming alternate names for that slot. The constraints contributed by
each signature retain their own scopes and are applied to the matched cells
when a call constructs its activation.

For example, a bodyless signature can name a positional-only implementation:

```cue
@experiment(aliasv2)
@experiment(functions)

_add1: func(_~x: int) -> int: x + 1
_add1: func(x: int) -> int

x: _add1(x: 2) // 3
```

Unifying a function type with a function value *tightens* the value: the
result is the function value with the type's parameter constraints and
result constraint enforced on every call. Each parameter of the type must
be declared by the value unless it is optional or declares a default, and,
unless the type is open, each parameter of the value must be declared by
the type or be optional or declare a default: a call through the type
never needs to bind such a parameter.

Whether a constraint of the type is compatible with the value is not
decided at unification time: constraints are enforced when the tightened
value is called, in the scope in which the type was declared. This
deferred checking is deliberate. It lets a user write an idealized
signature — say `func(int) -> int` — and bind an implementation with
stricter bounds, such as one backed by a 64-bit integer; a call within
bounds succeeds, and a call outside them fails with a clear `_|_`. This
matches CUE's standard data-validation model, where constraints are
checked against the values that actually flow through them.

Builtin raw metadata seeds unlabeled positional slots; the standard library's
generated CUE signature may name them. An anonymous user type remains
compatible because it contributes no competing label, and labeled parameters
remain callable positionally. The generated label is compatible; a different
label on the same ordinal conflicts. Calls reorder labeled arguments into the
builtin's raw positional sequence before invocation. A default declared by
an attached type is ignored on a builtin: a builtin's own defaults belong to
its implementation and keep deciding which arguments may be omitted, and a
declared default neither fills an omitted argument nor conflicts with the
builtin's own — the two are different mechanisms. This exception concerns
argument completion; it must not erase a conflict between attached function
types. In particular, two attached types with conflicting declared defaults
must remain incompatible when attached to a builtin.

```cue
@experiment(functions)

T: func(a: int, ...) -> number // open: admits further parameters
U: func(a: int) -> number      // closed

f: T & (func(a: int, b: int) -> int: a + b)
x: f(1, 2) // 3
y: f(1)    // _|_ // missing argument b

g:  U & (func(a: int) -> int: a)          // exact match
g1: g(1)                                  // 1
h:  U & (func(a: int, b?: int) -> int: a) // extra parameter is optional
hd: U & (func(a: int, b: int = 2) -> int: a + b) // extra parameter has a default
h1: hd(1)                                 // 3
e1: U & (func(b: int) -> int: b)          // _|_ // U declares a, the value does not
e2: U & (func(a: int, b: int) -> int: a)  // _|_ // b not admitted by closed U

// Constraints are enforced per call, in the type's scope.
s:  T & (func(a: string) -> int: 1)
e3: s(1) // _|_ // conflicting values int and string
```

#### Higher-order functions

A function type may appear anywhere a value may, including as a
parameter constraint, so higher-order functions need no additional
machinery:

```cue
@experiment(functions)

apply: func(f: func(int) -> int, x: int) -> int: f(x)
double: func(a: int) -> int: a * 2
r: apply(double, 3) // 6
```

As a convention — not an enforced rule — a higher-order function should
declare its function-typed parameters with anonymous positional
parameters (`func(int) -> int`). Any function whose parameters are
positionally callable then fits, regardless of what names the passed
function uses internally: the type's anonymous parameter matches the
value's parameter by position, and the passed function's own parameter
names are irrelevant at that call site. This gives the effect that
Swift achieves by disallowing labels on closure arguments, without a
dedicated rule.

#### Partial application

A call whose argument list ends in `...` is a partial application: it
binds the given arguments and yields a function over the remaining
parameters instead of evaluating the body. The bound arguments are
retained, each with the environment it resolved in; a later call
combines them with its own arguments, and the body is evaluated once no
parameter is left unbound. Partial applications may be chained.

```cue
@experiment(functions)

add: func(a: int, b: int, c: int) -> int: a + b + c

f: add(1, ...)          // a function of b and c
g: f(2, ...)            // a function of c
x: g(3)                 // 6
y: add(a: 1, ...)(2, 3) // 6
```

The trailing `...` is required: without it, an under-supplied call to a
function with required parameters is an error, so ordinary calls stay
strict. Parameter constraints are enforced across a partial
application, so an out-of-bounds bound argument fails at the partial
call. A bound parameter loses its default; the remaining parameters keep
theirs, which apply when the completing call leaves them unbound. Two
partial applications are equal only when they bound the same
arguments, so `add(1, ...)` and `add(2, ...)` do not unify, and neither
subsumes the other.

Partial application is the general form of a pattern CUE already uses:
validators such as `strings.MinRunes(3)` are builtins partially applied
to trailing configuration arguments while leaving the first, validated-value
slot unbound. Partial application of builtins themselves through `...` is not
supported at this stage and reports an error.

Function types must be attached before partial application; attaching one to
an already partial value is an error. Earlier types remain in force.

#### Recursion

Recursion, direct or mutual, does not terminate in finite structure and
results in a structural cycle error. Calls do not otherwise constitute
cycles: calls may be nested and repeated, including nested calls to the
same function.

```cue
@experiment(functions)

fib: func(n: int) -> int: fib(n-1) + fib(n-2)
f:   fib(5) // _|_ // structural cycle

twice: func(n: int) -> int: n + n
t:     twice(twice(twice(2))) // 16
```

This is a direct consequence of grounding functions in the existing
evaluation model: recursive re-entry through the same call site is
caught by the same structural cycle detector that bounds recursive
struct references today, while distinct call sites nest freely. A
recursive disjunct in a function body, such as `n | f(n)`, is likewise
a structural cycle and is eliminated from the disjunction. Functions
therefore do not extend the computational power of the language: CUE
remains non-Turing-complete, which fits its philosophy of separating
code from data while still allowing simple transformations without
contortions.

#### Relation to structs

The parameter list is deliberately a restricted struct body, and the
call activation preserves familiar struct reasoning:

*   an omitted optional parameter remains an optional field constraint;
    a supplied argument or used default defines a parameter's value;
*   a parameter default is declared, not inferred: `a: int = 5` is used
    as the argument when a call omits `a`, and a supplied argument makes
    it irrelevant; a default reached through a reference is an ordinary
    part of the constraint and never makes the parameter omittable;
*   parameter constraints meet on the same cell regardless of how the
    argument was bound, and the body meets the result constraint;
*   the implementation's finite parameter surface determines which
    arguments can be bound; argument checks and closing the generated
    frame prevent additions to that surface.

The struct model describes an admitted call's evaluation. Signature
matching, default completion, and opaque identity remain explicit parts
of the function design. In particular, agreement on the `!` marker is
a signature rule, and backwards compatibility compares calls rather than
performing one subsumption check on a frame. A native evaluator can
implement the same parameter and result unifications directly, without
materializing the illustrated fields.

#### Signature evolution and backwards compatibility

Modules evolve, and the signatures they export evolve with them. The
question "can I make this change without breaking my callers?" needs a
precise answer.

Compatibility compares the calling contracts of two signatures. A new
signature is a backwards-compatible evolution of an old one if

1.  **every call the old signature accepted is accepted by the new
    one** — for every call site that was valid, each argument still
    binds the corresponding parameter and satisfies its constraint
    (inputs may only widen), and
2.  **every result of the new signature is an instance of the old
    result constraint** — code consuming the result as a value of the
    old result type keeps working (the result may only narrow).

For matched parameters with old and new constraints `Pold` and `Pnew`,
the input condition is `Pold ⊑ Pnew`: `Pnew` subsumes `Pold`. For result
constraints it is `Rnew ⊑ Rold`: `Rold` subsumes `Rnew`. Binding modes,
labels, and omittability must also preserve all old calls. The two
subsumption checks run in opposite directions, so compatibility is not
subsumption of the entire illustrative frame.

For example, changing `func(x: int) -> number` to
`func(x: number) -> int` widens the input and narrows the result; it is
compatible by this rule even though neither ordinary frame struct
subsumes the other.

The table follows from these conditions. Additions and mode changes
must preserve existing positional ordinals and label bindings. Hidden-label
changes below are judged for callers outside the declaring package;
callers inside it can depend on those labels.

| Change | Why | Verdict |
| --- | --- | --- |
| widen a parameter type (`int32` → `int`) | new accepts a superset of calls | compatible |
| narrow the result (`int` → `int32`) | results remain instances of `int` | compatible |
| add an optional or defaulted parameter without shifting existing positions | old calls simply omit it | compatible |
| add a default to a parameter | old calls keep supplying it | compatible |
| change a default's value | old calls still bind; results of calls that omit the parameter change | compatible by the rule; a lint should flag it |
| remove a default from a parameter | old calls that omitted it fail | breaking |
| relax required to optional (`a!:` → `a?:`) | new accepts a superset of calls | compatible |
| make a name-only parameter positional (`a!: T` → `a: T`; `a?: T` → `a: T = d`) | by-name calls still match; positional calls are added | compatible |
| rename a positional-only alias (`_~x` → `_~y`) | the alias is internal, not contract | compatible |
| rename a hidden parameter name (`_a` → `_b`) | a hidden label binds only within the declaring package | compatible for external callers |
| make a hidden name visible (`_a: T` → `a: T`) | adds by-label calling for other packages | compatible for external callers |
| rename a named or name-only parameter | old `f(name: …)` calls no longer match | breaking |
| reorder positional parameters | old positional calls bind different parameters | breaking |
| remove a parameter, or add one that cannot be omitted | old calls fail | breaking |
| widen the result | results may leave the old result type | breaking |

Names and positions are one-way commitments. Making a name-only
parameter positional, after the existing positional parameters, *adds*
a way to call it: existing by-name calls keep working, and positional
calls become newly possible. An optional name-only parameter must gain a
default as it becomes positional (`a?: T` → `a: T = d`), because a
bare positional parameter must be supplied; without the default,
callers who omitted it would break. The reverse moves — taking a
position or a name away — are breaking. This is the reverse of
Python's situation: Python started with every parameter callable both
ways and could thereafter only restrict (`/` and `*`), which breaks
callers; CUE starts each parameter at the form its author chose and
can only widen.

Two honest limitations of the rule:

*   It is a statement about the *interface*, not about any particular
    bound implementation. Narrowing a result constraint is compatible
    for consumers of the result, but if an implementation is bound
    whose body does not meet the narrower constraint, calls that used
    to succeed will now fail — at the call, by the deferred checking of
    [Function types](#function-types). A compatibility check tells you
    your callers still typecheck; it does not prove every call still
    succeeds.
*   A checking tool should stay conservative: report "compatible" only
    when preservation of bindings and both subsumption conditions are
    certain, and flag for review otherwise.

This interface comparison intentionally ignores a default's value while
preserving whether omission is allowed. Signature unification does not
ignore the value: `func(x: int = 2)` and `func(x: int = 3)` have compatible
interfaces, but their defaults conflict under `&`. Mutual interface
compatibility therefore does not imply equality as CUE values. Likewise,
removing an optional parameter breaks calls that supplied it, even though
no call was required to supply it.

The experimental function-subsumption implementation is not yet a complete
implementation of this compatibility check, nor does the frame illustration
establish its lattice laws. A compatibility tool can reuse CUE subsumption
for the input and result constraints after checking bindings. Surfacing that
tool in the CLI is left to a follow-up.

#### Native implementation notes

These non-normative notes connect the explanatory activation model to the
experimental evaluator and record its current limits.

The experimental implementation evaluates functions natively:

*   A call is evaluated through an activation holding one arc per
    parameter, seeded with the argument values, below which the body is
    evaluated. The result of the call is the unification of the body
    and the result constraint.
*   Finalized, error-free call results are memoized per call site and
    calling environment, so a call's result is evaluated once rather
    than once per consumer. This keeps the cost of deeply nested calls
    linear rather than exponential; a benchmark guards the property.
    Incomplete results are never memoized.
*   Recursion detection reuses the existing structural cycle detector:
    each function anchors recursion detection at a stable per-closure
    point, so recursive re-entry is caught while distinct call sites
    nest freely.
*   Exporting a function that captures fields of enclosing structs
    re-anchors the literal at its emission position, so exported
    configurations remain self-contained. A partially applied value
    currently exports as its underlying function literal; round-tripping
    the bound arguments through export is left for later.
*   Dependency analysis and `cue trim` traverse the expressions of
    function literals like other compound values.
*   A parameter default is compiled in the closure scope, like the
    constraint, and is added to the parameter's activation arc when the
    call is scheduled, with the same anchored cycle context, so a default
    that calls the function itself is a structural cycle. Each default is
    checked against its constraint once the value declaring the literal
    has been evaluated, so that a default referring back to the function
    does not disturb a call that supplies the argument; a default that is
    not yet resolvable leaves the check undone. A default
    contributed only by an attached type is not part of the exported
    literal; it stays in the `& type` term that export emits alongside it.
*   Attributes attached to a parameter are parsed and formatted but not
    yet read by the compiler.

**Composition and closed types.** Unification must be associative,
commutative, and idempotent, and unifying with bottom must remain bottom.
Admission beyond a closed type therefore depends on the value's own
declaration alone: a closed type restricts the value it is attached to
whatever else is attached, as a closed struct restricts its fields, and a
default that only another attached type declares does not admit the
parameter. For example:

```cue
@experiment(functions)

f: func(x: int) -> int: x
D: func(x: int = 2) -> int
T: func() -> int

a: ((f & D) & T)() // _|_: parameter x not allowed by the closed T
b: ((f & T) & D)() // _|_: the same, whatever the order
c: D & T           // admitted: D's own default makes x omittable
```

An earlier revision admitted `a` because `D` had been attached first,
which made the outcome depend on operand order; saying that fragments are
normalized before closing did not specify a rule either way. `D`'s default
still fills `x` for calls through `f & D`; it is the closed `T` that
`f & D` cannot satisfy. Likewise, attaching types to a builtin preserves
conflicts between their declared defaults even though those defaults never
supply a builtin argument: `strings.Repeat & T1 & T2` with differing
defaults for the same parameter is a conflict, as `T1 & T2` is.

One implementation gap remains. Function subsumption ignores a default's
value, as the compatibility check requires, so it can report mutual
subsumption for two types whose defaults make their meet bottom.
Resolving it means separating the lattice order, which must compare
default values, from the interface comparison, which must not. The struct
illustration explains activation evaluation and does not claim to prove
the soundness of every function operation.

### Migration

A CUE program can span many modules, each written against a different
language version, and these must coexist. The feature is therefore
introduced through CUE's per-file experiment mechanism, with a path to
eventual adoption in a language version.

**Per-file opt-in.** The syntax is enabled per file with
`@experiment(functions)`. A file without the attribute parses exactly
as it does today:

*   `func` remains an ordinary identifier there — the parser
    distinguishes a function literal from a call to a field named
    `func` only when the experiment is enabled, using lookahead that is
    free of side effects;
*   the scanner produces the `->` token only when the experiment is
    enabled, so no existing token sequence changes meaning;
*   labeled arguments in calls are likewise gated.

Because gating is at the file level, a module using functions can
depend on — and be depended on by — modules that know nothing about
them. Function values are ordinary values at evaluation time; only the
syntax is gated.

**Graduation to a language version.** When the experiment graduates,
the syntax becomes available to any file whose module declares a
sufficient language version, and the attribute becomes redundant.
`cue fix` can then remove the now-redundant `@experiment(functions)`
attributes as part of a module's version bump.

**Reserving the keyword.** `func` is initially a contextual keyword. If
it later becomes fully reserved — a breaking change tied to a language
version — `cue fix` can mechanically migrate affected code, rewriting
regular fields named `func` to their quoted form and updating
references:

```cue
// before
s:   {func: exec.Run}
run: s.func

// after cue fix
s:   {"func": exec.Run}
run: s."func"
```

Occurrences of `func` as a label in string form, in selectors, and in
imports are all rewritable by syntactic rules alone, so the transition
can be executed automatically per file once its module opts into the
new language version. Files under older language versions are
untouched and continue to parse with `func` as an identifier.

**Migrating struct-simulated functions.** No automatic rewrite is
proposed for the existing struct-as-function pattern: it is a
convention, not a recognizable construct, and it remains valid CUE.
Authors can migrate incrementally, typically turning

```cue
sum: {a: int, b: int, out: a + b}
r:   (sum & {a: 1, b: 2}).out
```

into

```cue
sum: func(a: int, b: int) -> int: a + b
r:   sum(1, 2)
```

### Future extensions

These are deliberately left out for now and addable later without
breaking existing code — preferring a minimal core to which features
are added only once demand justifies them.

#### Foreign function interfaces, purity, and side effects

A primary driver for functions in CUE is the ability to securely and
predictably bind to foreign implementations: builtins, Protobuf
services, or host-language functions. The signature language in this
proposal is designed with that in mind — parameter attributes give
bindings a natural home, and deferred constraint checking bridges CUE's
arbitrary-precision numbers to hardware-bounded foreign types (a
signature `func(int) -> int` bound to a 64-bit implementation enforces
the 64-bit bounds per call).

The semantics of foreign calls raise questions this proposal does not
answer: how purity is declared and what may be assumed of an unmarked
function, how side effects interact with CUE's assumption that
evaluation order is irrelevant, whether impure calls may appear in
disjunctions, and how execution is requested. The intended direction is
secure by default — foreign functions assumed impure unless marked
pure, with impure evaluation strictly contained — but the design is
deferred to a follow-up proposal.

#### Dependent parameter types

Referring to one parameter from another parameter's constraint or from
the result constraint is reserved as an error rather than resolving to
an enclosing scope. This keeps the door open for dependent parameter
types, which could express input/output relationships:

```cue
append: func(list: [..._T], elem: _T, _T: *_ | _) -> [..._T]: [for x in list {x}, elem]
```

Such signatures would approximate generics. The conceptual complexity
outweighs the benefit at this stage; the reservation ensures the
extension remains possible without changing the meaning of any valid
program.

#### Overloading

Because function values live in the lattice, a disjunction of functions
is expressible, and calling it amounts to overloading: a call would
distribute over the disjunction, eliminating branches that fail.
Currently, calling an unresolved disjunction of function values is
incomplete unless a default resolves it. Full overload resolution —
including its interaction with non-concrete arguments, where a branch
may remain incomplete rather than eliminated — is left for a future
proposal.

#### Variadic arguments

A variable number of arguments is passed as a list — `func([...string])
-> string`, called as `f(["a", "b", "c"])`. This is exactly how the
"variadic" builtins already work today (`and`, `or`, `list.Concat`, and
`strings.Join` each take one list), and for a data-centric language a
pair of brackets is the native idiom rather than friction. Native
variadic parameters are excluded for now; see
[Alternatives Considered](#why-native-variadic-parameters-are-excluded)
for the rationale. A small set of reflective builtins whose calling
conventions cannot be expressed as user functions — similar to Go's
`append` — remain builtin-only, documented with illustrative
signatures.

#### Unifying function implementations

Unifying two distinct function values currently yields `_|_`. A
possible extension is to define `f & g` as the function that evaluates
`f(args) & g(args)` — one definition enforcing defaults or policy
while another supplies core logic. This composes well with pure
functions but interacts subtly with future impure FFI, so it is
deferred to the FFI follow-up.

#### External versus internal parameter names

Swift separates a parameter's external label from its internal name.
The alias syntax generalizes naturally — `label~name!: T` would fix the
contract label while leaving the body name freely renameable — but only
the positional-only form `_~name` is part of this proposal.

### Alternatives Considered

#### Reusing disjunction defaults for parameters

An earlier revision of this proposal reused CUE's disjunction defaults
for parameters: `pick: func(a: int | *5)` could be called as `pick()`,
and a parameter could be omitted whenever its constraint resolved to a
single default. Three defects made this untenable. A supplied argument
did not displace the default: the parameter was the unification of
constraint and argument, so passing a value that carried its own default
— a common case, since callers pass fields that are themselves defaulted
— unified two defaults, and when they differed both canceled, leaving a
bare disjunction and an incomplete result instead of the caller's value.
Omission was an analysis of the constraint rather than a declaration,
decidable only at call time, which is why a defaulted parameter could not
count as omittable when a closed type was attached, and why an omitted
parameter evaluated to the whole disjunction (`int | *5`) rather than to
the default. And the default was inseparable from the type: there was no
way to say "this parameter is `int`, and `2` if not supplied" without the
disjunction flowing into the body and out through the result. The
declared default of this proposal fixes all three: omission is declared,
a supplied argument makes the default irrelevant, and the body sees only
the value the default produced. A mark written in a parameter's
constraint is rejected so that the retired idiom fails loudly rather than
silently changing meaning.

#### Deriving a variant with a different default

Two ways to derive a function with a different default were considered
and set aside. By unification: two signatures that declare different
defaults for the same parameter conflict, because unification is
commutative and no side could override the other; cementing the
drop-on-supply semantics mattered more than this convenience. By a
re-defaulting argument form in partial application, `f(b = 3, ...)`,
leaving `b` overridable in the derived function: this would be new call
syntax and was judged not worth adding at this point. A wrapper function
that declares the new default covers the need.

#### A separate function type system

Modeling parameter and result constraints in a type system independent of
CUE's value lattice was rejected: existing CUE constraints already express
the values a call may accept and produce. Restricted struct declarations
provide the parameter syntax, and activations reuse field unification.
Functions still need explicit rules for their calling contracts, defaults,
and identity. Evolution reuses subsumption of the input and result
constraints, in opposite directions, after checking argument bindings.

#### Independently closed signature structs

An earlier form of the structural model translated every signature
independently:
`func(a: int, b: int) -> int` mapped to a closed struct with a hidden
positional list and per-parameter mirror fields, a call mapped to a
plain struct of arguments, and application was unification with a
result read back from a hidden field. The encoding had attractive
properties — a call could be encoded without consulting the signature,
and signature composition was literally struct unification — and a
prototype implemented it.

That prototype was replaced by native evaluation: its mirrors only flowed
from position to label, and each closed signature rejected labels contributed
by the others. The shared-slot illustration in this proposal explains how
the native evaluator matches labels and applies constraints to a call's
activation. Recursive closing belongs to that generated activation; it does
not mean that independently closed signature structs can be unified to
derive every function rule.

#### Named parameters callable both ways

A simpler model was seriously considered in which a parameter is either
anonymous-positional or name-only, never both. It needs no
name-position bookkeeping at all. But it lands the everyday case in an
awkward place: in `func(a: int, b: int) -> int: a + b`, the natural
two-argument function, `Sum(1, 2)` would be rejected, and recovering
positional calling would require the verbose `func(_~a: int, _~b: int)`
form. In nearly every language users know — Python, Go, Java, C#,
JavaScript, Rust, Kotlin, Ruby, Ada — a plainly declared two-parameter
function is callable positionally.

The chosen design makes a bare `a:` callable both ways and treats any
contract name as final. This is deliberately Python-like in
convenience, but removes the trap by making the commitment explicit:
naming a parameter is permanent, with `a!:`/`a?:` to opt out of
positional calling and `_~a` for a renameable, non-contract name.
Notably, the restrictive model could have been widened into this one
later without breaking any caller — the reverse of Python's path, which
started permissive and could only restrict, breaking callers each time.

#### Graceful degradation on name mismatch

For unification of function types, an alternative was considered in
which two types naming the same position differently would not fail but
degrade: the positional structure would survive and only the clashing
names become unusable. This is arguably the exact lattice meet — a call
is valid for `f & g` iff both accept it — and it makes name mismatches
in higher-order composition maximally forgiving.

The current design instead makes `func(a: int) & func(b: int)` a conflict:
one positional slot cannot have two contract labels. Errors are loud and
simple to explain, and the forgiving behavior is recovered by convention:
higher-order signatures declare their function-typed parameters with
anonymous parameters, which match any positionally callable function
regardless of its names. Relaxing an error into degradation later is a
widening move; starting with degradation and tightening later would
break code that relied on it.

#### Why native variadic parameters are excluded

Every "variadic" builtin today is a single list parameter, and the
evaluator has no variadic concept — native variadic parameters would be
a new subsystem modeling something the engine has never needed. They
also uniquely complicate the relationship between a call and its
signature: with variadics, which arguments gather into the variadic
list can only be determined by consulting the signature. Lists cover
the ordinary cases idiomatically. This may be revisited if the added
complexity proves worthwhile.

#### Letting foreign functions execute freely

Rejected in advance of the FFI follow-up: CUE's evaluation model
assumes the result is independent of evaluation order, and unrestricted
side effects during unification or disjunction exploration would make
evaluation unpredictable. Any FFI design will be secure by default;
see [Future extensions](#foreign-function-interfaces-purity-and-side-effects).

### Cross-cutting Concerns

#### Security

Native CUE functions have no side-effect primitives and are pure by
construction; adding them does not enlarge what a configuration can do
to its environment. The deferred FFI proposal is where execution enters
the picture, and it will be gated and impure-by-default. A more
comprehensive capability model should be considered there.

#### Performance

Call results are memoized per call site and calling environment, so
sharing a computed result across many consumers does not repeat work,
and deeply nested calls evaluate in linear rather than exponential
time. Recursion is cut off by the structural cycle detector rather
than by depth or fuel limits, keeping evaluation deterministic.

#### Tooling

`cue fmt` formats function literals and calls; `cue fix` automates the
migrations described under [Migration](#migration). Dependency
analysis, `cue trim`, and export handle function values. A
compatibility check based on argument binding and input/result subsumption
(`cue vet`-style) is a natural follow-up, as is a lint for signature
changes that are compatible by the rule but risky in practice.

### Comparison with other languages

**Python.** Parameters are positional-or-keyword by default, so names
leak into the contract implicitly, and renaming or reordering silently
breaks keyword callers. This design is deliberately Python-like in
letting a parameter be called by name or position, but makes the
contract explicit and final, with a name-only marker (`a!:`/`a?:`) to
opt out of positional calling and an alias (`_~a`) for a renameable,
non-contract name. Annotated Python writes a default exactly as CUE does
(`b: int = 2`), and keyword-only defaults (`*, t=30`) correspond to
`t!: int = 30`. Python evaluates a default once, at definition time, and
a default that names another parameter silently binds an enclosing
variable of that name; CUE resolves defaults in the same scope but makes
the reference to a sibling parameter an error, and the once-versus-per-call
distinction has no observable counterpart for immutable values. Python
requires defaulted parameters to trail the required ones; CUE, like Swift
and Kotlin, lets a required parameter follow a defaulted one, since it can
be bound by label.

**Swift.** Swift's external-label/internal-name split gives call-site
clarity and free internal renaming; this design provides the internal
name through the `~` alias, without Swift's mandatory labels. Swift
disallows labels on closure arguments; CUE reaches the same outcome
through anonymous parameters in higher-order signatures. Swift declares
defaults as CUE does, checks them against the parameter type at the
declaration, and rejects a default that refers to another parameter —
the same three choices. One departure: a Swift function type carries no
defaults, so calling through one requires every argument, whereas a CUE
function type may declare defaults, because a signature is a value and
builtin signatures are written as types.

**JavaScript and TypeScript.** A default fires for an explicit
`undefined` argument, a widely documented surprise. CUE sits with Python,
Swift, and Ruby instead: any supplied argument, including `_`, displaces
the default. TypeScript forbids combining `?` with an initializer because
its `?` means only "optional"; CUE agrees, since an omitted `?` parameter
is absent from the body, and writes the name-only default on the required
marker: `b!: int = 2` is `b: int = 2` minus positional binding.

**C#.** Interface methods may declare defaults and implementations may
declare different ones, and which applies depends on the static type at
the call site. CUE's rule that two-sided defaults must unify, and Kotlin's
rule that an override cannot redeclare a default, both avoid that
ambiguity.

**Go.** Go has no named arguments or defaults, so optional
configuration goes through options structs or functional options; CUE's
name-only parameters and `= d` defaults express this directly, and
adding a name-only parameter with a default preserves existing calls.

**Ada.** The both-ways default and the positional-first call rule date
back to Ada 83 — every parameter callable by position or by name, with
a named argument ending the positional run. Ada lacks the opt-outs this
design adds.

**Rust.** Rust has no named arguments, no defaults, no overloading: a
parameter name is a pure local. The price is the builder pattern. This
design takes the opposite wager — names as contract by default — with
`_~a` recovering Rust-style safe renaming exactly where wanted.
