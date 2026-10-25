# Function Contracts: Signatures That Bind Callers and Implementations

## A revision of the CUE functions experiment

> **Status: experiment.** Functions are gated behind `@experiment(functions)`.
> This document revises the semantics of that experiment in place, and most of
> it is implemented in the development version (section 10).
> Code marked `cue` uses the experiment's syntax. `func!`, `func?`, and
> `func[...]` are proposed syntax, and arrows in plain fences are notation.

**Builds on.** This document continues the
[functions proposal](https://github.com/cue-lang/proposal/blob/main/designs/language/4484-functions.md)
and its
[theory addendum](https://github.com/cue-lang/proposal/blob/main/designs/language/4484-theory-addendum.md),
*Why CUE Functions Tighten Instead of Widen*.
The addendum chose to read a signature as a constraint on calls and noted what
that reading lacks: a plain signature does not promise that an implementation
covers its domain.
This revision adds that promise.
It keeps the constraint on callers and adds the check the addendum left open, so
that a signature now binds both its callers and its implementations.

Aram Hăvărneanu's Quantified CUE counterproposal
([discussion 4492](https://github.com/cue-lang/cue/discussions/4492)) and its
accompanying paper showed that standard contravariance can be applied to CUE
functions while meeting the addendum's requirements, all but one by the
counterproposal's own account: it is intentionally more powerful than a struct
(requirement 6).
This revision closes that gap.
It applies contravariance without adding power, and it keeps the constraint on
callers (section 2).
It also adopts several of the counterproposal's principles.
An implementation is checked against every interface it is given, where the two
meet.
That check does not depend on where declarations are written or how they are
grouped.
A check never rewrites the implementation it checks.
A successful check means partial correctness, not that every call succeeds.
Parametric functions, when they come, follow the direction of its quantified
functions.
We thank Aram for the counterproposal and the paper: this revision is
substantially better for them.

**In one paragraph.** Implementations are now checked the way function subtyping
prescribes: an implementation unified with a signature must accept every
argument the signature admits and must declare a result the signature allows,
contravariant in parameters and covariant in results.
The check runs where the two meet, whether or not the function is ever called,
and each implementation is checked against each signature on its own.
Callers stay restricted covariantly, as `&` does for every other field: a wider
implementation never widens the calls a signature permits.
Every function field has callers and an implementation, and a signature binds
both, so the check needs no role.
The field marker says which party is which, and that matters only for evolving
an interface.
Functions passed as arguments are checked where they are bound, Go functions
declare their signatures through a default integer mapping that a registration
can override, and two distinct implementations no longer unify.
Only a definite incompatibility rejects.
Whatever the check cannot decide is left to the call, which enforces the
signatures regardless.

---

## 1. Implementations are checked contravariantly

An implementation unified with a signature must accept every argument the
signature admits and must declare a result the signature allows.
This is the standard rule of the addendum's section 2: contravariant in
parameters, covariant in results.

```cue
f: func(x: number) -> string
f: func(x: int) -> string: "int"
// f: parameter x of function accepts int, but its signature requires number

R: func(x: int) -> int
W: func(x: int) -> number: 1
w: R & W
// w: function returns number, but its signature requires int
```

The check compares declarations, not behavior: `W`'s body returns `1`, an
integer, but its declared result `number` is not within `int`, and no observed
result repairs a declaration.
The check runs where an implementation and a signature meet, whether or not the
function is called, and reports the positions of both.
It applies to every signature an implementation is unified with, so
`func(_) -> _` requires an implementation that accepts any value.
It is not a wildcard for "any function".
Builtins are checked at the level of their parameter and result kinds.

This is the obligation of the addendum's section 6.2, with the result check that
section left open, and with one change: it is on by default.
Every bodiless signature carries it.
Section 2 explains why that does not guess a role.

---

## 2. Callers stay restricted covariantly

The check does not loosen what a signature says about calls.
Several signatures meet covariantly, as before, and a wider implementation does
not widen the calls a signature permits:

```cue
#f: func(x: int) -> int
#f: func(x: number) -> int: 2   // admitted: it accepts every int
a: #f(1)                        // 2
b: #f(1.5)                      // _|_: conflicting values 1.5 and int
```

Under an annotation-style contract reading `b` would be `2`: a contract binds
whoever supplies the body and nobody else (addendum, 3.3).
This is where the design differs most from Quantified CUE, whose ordinary arrows
do not retain a constraint on callers.

**A signature binds both sides because every function field has both.** Someone
calls the function and someone implements it.
For a regular field the definer implements it and users call it.
For a `!` or `?` field users implement it and the module calls it (section 8).
Either way the signature limits the calls and obliges the implementation, so the
check is the same whichever party is which.
This also answers the addendum's objection to a contract reading of `&`, that it
must guess the role: checking both sides is correct for either.
The field marker matters only where the two parties want different things, in
deciding whether a new version of an interface can replace an old one.
The addendum's section 4 shows that a function field is invariant under
replacement unless that role is known.

The price is the case the addendum calls spurious: a bodiless signature meant
only to restrict callers now also obliges its implementation.
That is the right default for the main use of a CUE signature, describing code
implemented elsewhere (addendum, section 1).
The caller-only projection of section 9 would express the exception.

---

## 3. Combining signatures

Each bodiless signature contributes a caller constraint `C` and a provider
requirement `R`.
An implementation contributes only its own declaration `D`, which never changes
`C` or `R`.
Conjunction keeps every contribution: a call must satisfy every caller
constraint, and an implementation must meet every requirement.
For complete clauses without dependencies, writing a clause as its input domain
and result bound `(A, B)`, the contributions combine into one caller constraint
and one requirement:

```
C1 & C2  =  (A1 & A2, B1 & B2)      callers get the intersection
R1 ⊕ R2  =  (A1 | A2, B1 & B2)      implementations owe the union
```

For results there is no difference: on both sides a result must satisfy every
signature.
Only the parameters combine differently, and that difference is the variance:

```cue
c:    (func(x: int, y: string) -> int) & (func(x: number, y: "a") -> int)
cOK:  c & (func(x: number, y: string) -> int: 1)   // admitted
cBad: c & (func(x: int, y: string) -> int: 1)      // _|_: does not accept every number for x
cy:   cOK(1.5, "a")                                // _|_: callers may pass only an int for x
```

Callers may pass only `x: int, y: "a"`, while an implementation must accept
`x: number, y: string`.
Combining parameter by parameter loses nothing, because an implementation
declares each parameter separately: accepting the arguments of both signatures
is the same as accepting, for each parameter, what either signature allows.
The combined form is only a shortcut for signatures that are fully known.
The evaluator keeps the list of signatures and checks them one by one while
anything is still being evaluated.

Keeping `R` separate is what keeps `&` associative where the addendum's Hybrid B
was not (3.3):

```cue
I: func(x: int) -> string
N: func(x: number) -> string
b: func(x: int) -> string: "int"
r1: I & (N & b)   // _|_: b does not cover number
r2: (I & N) & b   // _|_: the same; I & N keeps N's requirement
```

A number-capable implementation passes in every grouping, and calls through it
still accept only integers.

---

## 4. Functions passed as arguments

A function passed to a parameter typed by a signature is checked where it is
bound, in two respects: its body and its own signatures must each accept every
argument the parameter's signature allows.
First, its implementation must cover the parameter's signature (section 1).
This recovers the modular check whose absence the addendum named as the cost of
covariance (6.1):

```cue
apply: func(f: func(number) -> number) -> number: f(1.5)
ints:  func(x: int) -> number: x
bad:   apply(ints)
// bad: parameter 1 of function accepts int, but its signature requires number
```

Second, its own signatures must permit every call the parameter entitles the
callee to make.
A callback with a number-capable body behind an int-only signature cannot be
passed where numbers will be supplied, whether or not the callee makes such a
call:

```cue
#Cb:   func(x: int) -> int
cb:    #Cb & (func(x: number) -> int: 1)
quiet: func(g: func(number) -> int) -> int: 7
q:     quiet(cb)
// q: parameter x of the function supplied for parameter g permits only int, but parameter g permits number
```

Conjunction cannot produce this second check: unifying the parameter's signature
with `cb` only narrows it to `int` and moves the failure into the callee's body.
It is a contravariant comparison of caller constraints, made by the call as it
binds its arguments, which is the call-time variance check the addendum's open
questions asked about.
Only arguments, defaults, and signatures written as literals or references are
inspected there.
Anything computed by an expression is left to the call, which still enforces the
signatures.

Both checks compare a parameter that is itself a function type as a function
type, so variance flips at each level of nesting, as in standard function
subtyping:

```cue
sig:  func(f: func(int) -> int) -> int
impl: func(f: func(number) -> int) -> int: f(1.5)
s:    sig & impl
// s: parameter f of function accepts func(number) -> int, but its signature requires func(int) -> int
```

Callers of `sig` may pass a callback that accepts only integers, which `impl`
would call with `1.5`.
With the parameter types the other way around, the implementation is admitted.
One level deeper the direction flips back, and results compare covariantly at
every level.

---

## 5. What a check establishes

The check rejects only what it can show to be wrong.
Some comparisons CUE cannot decide exactly, such as whether one regular
expression covers another.
In those cases the implementation is admitted, and the signatures still check
every call:

```cue
re: func(s: =~"^a$") -> int
re: func(s: =~"^[a-z]+$") -> int: 1   // admitted: the patterns are not compared
x:  re("b")                           // _|_: "b" does not match =~"^a$"
```

A mismatch needs evidence: a kind of value that the implementation excludes, or
a concrete value that the signature admits and the implementation rejects, found
among the values the two constraints mention.
So `"a" | "b"` covers `=~"^[ab]$"`, while an implementation that accepts only
integers never covers a pattern on strings.
Whatever the check cannot decide this way is admitted, such as a mismatch that
only an untried value would show, closedness, or a parameter typed by a
conjunction of signatures.
A constraint that refers to a value that may still be refined is left to the
call rather than judged on a provisional value.
This uses CUE's existing dependency handling and package boundaries, and adds no
way of freezing declarations.

A successful check means that the declarations fit: if a call succeeds, its
result lies within the signature.
It does not mean that every allowed call succeeds:

```cue
at: func(i: int) -> int
at: func(i: int) -> int: [10, 20, 30][i]   // admitted: it declares every int
y:  at(5)                                   // _|_: index out of range [5] with length 3
```

Nor does it mean that a body justifies its own declaration, or that a Go
function cannot overflow internally.

---

## 6. Distinct implementations do not unify

Two distinct function implementations no longer unify.
Unifying them is an error, reported where they meet with the positions of both,
as for two distinct concrete values such as `1 & 2`:

```cue
url: func(host: string) -> string: "http://\(host)"
url: func(host: string) -> string: "https://\(host)"
// url: conflicting function implementations: each function literal is a distinct value
```

A function still unifies with itself, with the same function reached by another
path, and with any number of bodiless signatures.
This reverses the choice of the addendum's sections 3.1 and 5.2, which kept
pointwise conjunction, `(f & g)(x) = f(x) & g(x)`, partly because rejecting
distinct bodies needs a notion of function identity.
In the experiment, pointwise conjunction raised four problems:

- **Conflicts surface late, or never.** With composition, the example above
  passes `cue vet`.
  Only a call fails, and a configuration that never calls `url` never reports
  the conflict.
  That runs against everything else in this document, which moves function
  errors to where a function is formed.
- **A disjunction of functions cannot choose.** With composition, `(f | g) & f`
  keeps two alternatives, `f` and `g & f`.
  Without it, `g & f` fails and the disjunction resolves to `f`.
- **Only some functions composed.** Builtins, Go functions, and partial
  applications already conflicted with other functions.
  Only CUE bodies composed.
- **The cost can multiply.** Every call runs every implementation, and the calls
  each implementation makes are evaluated separately.
  In a chain where every function has two identical implementations, the work
  can double at each level:

  ```cue
  f0: func(x: int) -> int: x
  f1: (func(x: int) -> int: f0(x)) & (func(x: int) -> int: f0(x))
  f2: (func(x: int) -> int: f1(x)) & (func(x: int) -> int: f1(x))
  // and so on, up to f12
  out: f12(1)
  ```

  In the experimental evaluator, such chains took:

  | Chain depth | One implementation | Two implementations |
  |---|---|---|
  | 8 | 0.3 s | 0.6 s |
  | 12 | 0.3 s | 71 s |
  | 16 | 0.4 s | did not finish within 300 s |

There is also a reading problem.
In type systems with intersection types, such as TypeScript's, `F & G` on two
function types is an overloaded function callable with either argument type, and
`F | G` is a function of one of the two types.
Giving `&` on function values the meaning "run both and meet the results" would
make `f & g` look like overloading while meaning something else.

**Identity.** Two function values are the same when they come from the same
function literal, with the same bound arguments for a partial application, and
refer to the same values outside the literal.
A function that captures nothing is therefore the same wherever it is evaluated,
and so is a closure wherever what it captures is equal.
The addendum's own examples behave as it required, here with a closure:

```cue
x: {k: 1, f: func(y: int) -> int: y + k}
a: x.f & x.f              // f
b: x.f & {x}.f            // f: k is 1 in both
c: x.f & (x & {g: 1}).f   // f
d: [for v in [1, 2] {func(y: int) -> int: y + v}]
e: d[0] & d[1]            // _|_: different captured v
```

A closure is compared by what it captures, not by where it was evaluated, so the
outcome does not depend on whether the evaluator shares a struct.
Two literals with the same text are distinct.
There is no structural or extensional equality of function literals.

**What to write instead.** Combining the results of two functions is explicit
and takes one line:

```cue
labels: func(name: string) -> {...}: base(name) & extra(name)
```

**What stays open.** Disallowing composition now keeps the meaning of `f & g`
available.
Allowing it later is backward compatible.
Withdrawing a shipped meaning would not be.
Candidates:

- **Pointwise meet**, the disabled behavior, for example for struct-returning
  templates contributed from several places, if its cost and its late
  diagnostics can be addressed.
- **Gluing partial implementations.** Each implementation covers part of the
  domain, the implementations agree where their domains overlap, and together
  they cover what the signatures require.
  A call uses the implementations whose domains contain the argument.
  This is the gluing condition of a sheaf: sections defined on parts of a domain
  that agree on their overlaps glue to a unique section on the union.
  It fits the provider side of section 3, where an implementation already owes
  the union of the required domains.
- **Alternatives with `|`.** Calling a disjunction of functions as
  `(f | g)(x) = f(x) | g(x)` would drop the alternatives that reject the
  argument, selecting an overload by elimination.
  Calling a disjunction of functions is not supported today.

---

## 7. Go functions

A Go function registered through the bridge is a builtin carrying a declaration
derived from its Go signature.
Registration is the evidence, which is the registration-time check the addendum
describes in section 3.2: attaching a signature checks the declaration like any
builtin's, and never calls Go.
The default integer mapping is:

| Go type | Declared CUE constraint |
|---|---|
| `int`, `int64` | `int` |
| `uint`, `uint64`, `uintptr` | `uint` |
| `int8`, `int16`, `int32`, `uint8`, `uint16`, `uint32` | the bound of the same name |
| other types | the kinds decoding accepts, or encoding produces for a result |

The defaults keep native limits out of public signatures in the common cases.
A Go `int64` function satisfies `func(int) -> int`.
An argument the native type cannot hold fails before the call, with an error
naming the Go type, and never wraps.
For Go `int` and `uint` the error states the target's width.
By default the narrow kinds are exposed exactly: a Go `func(int8) int8` cannot
satisfy `func(int) -> int`, since `int` is not within `int8`.
A registration can override the default, declaring for instance a Go `int32`
parameter as `int`.
The conversion is then checked in the same way.
Narrow widths thus stay visible unless a registration chooses otherwise.
Like the default, an override is part of the registration: attaching a signature
in CUE never widens a declaration.
A conversion check does not make Go arithmetic safe: an `int8` function can
still overflow after a valid conversion.
A Go function returning `bool` stays a function and is never treated as a
validator.

Bridge transports, the Go bridge among them, belong to the implementation.
This document concerns the declarations bridged functions carry.
A candidate notation for showing native types reuses the slots of the existing
`@go` attribute, Go name first and Go type second:

```cue
Foo: func(a: number, b: string) -> int @go(,func(float64, string) int64)
```

It would describe a registered declaration, never establish one, and is not
adopted.

---

## 8. Compatibility and migration

Pre-functions CUE is unchanged.
The `functions` experiment changes in place, without a new flag, because this is
the same design made stricter and the bindings it newly rejects are expected to
be genuine mismatches.
Newly rejected are:

- An implementation that declares less than a signature requires, even if never
  called.
- A callback whose implementation or own signatures fall short of the parameter
  it is bound to.
- Two distinct implementations of one function, which the experiment composed.

Errors carry the positions of both declarations, and an error arising in a call
is reported at the field holding the call.
The fix is an honest declaration, a wrapper, or a narrower interface.
Code that composed implementations combines their results in one body.

Whether a new version of an interface can replace an old one remains a job for
tooling, with the rules of the addendum's section 6.3.
Such a tool needs the role, and this revision adopts a convention that states it
without new syntax: a regular field `foo: func…` is a function its definer
provides, so its evolution follows the callers' direction.
A field `foo!:` or `foo?:` is one its user must or may supply, so it follows the
implementers' direction.
This matches what a required field already means: someone must still supply it.
Admission does not depend on the marker.

---

## 9. Future directions

- **Projections.** `func!(A) -> B` would contribute only the requirement and
  `func?(A) -> B` only the caller constraint, as opt-outs from the default that
  an ordinary signature contributes both.
  For example, `func!(number) -> string & func?(int) -> string` requires a
  number-capable implementation but permits only integer calls.
  Neither form can remove a constraint or requirement contributed elsewhere.
- **Combining implementations**, as listed in section 6.
- **Parametric functions**, such as `func[A: _, B: _](a: A, b: B) -> B`, in the
  direction of the quantified functions of Quantified CUE.
  The monomorphic checks of this document are meant to be the instance-level
  checks such a form would perform.
- **Body checks in `cue vet`.** A later analysis could establish, refute, or
  leave unresolved whether a body justifies its own declaration, starting with
  obvious result mismatches.
  It would never run Go or other host code to do so, and a stronger proof would
  never silently repair a declaration that fails its signature.
- **Export of function values**, either as a `func Foo(x) -> y` form or by
  omitting them from data exports.

---

## 10. Implementation status

The development version of the experiment implements the checks of sections 1 to
4, including builtins, callbacks, and nested function types.
It also implements the conflict rule and identity of section 6, the Go
declarations of section 7, and error reporting at the calling field.
A constraint that referred to a value still being refined is not re-checked yet.
Today the call checks it.
Registrations cannot yet override the default Go declarations of section 7.
The compatibility tooling of section 8, body checks, and the forms of section 9
do not exist yet.

---

## 11. Against the requirements for alternatives

The addendum's section 9 lists six requirements for any alternative design.
This revision meets them as follows:

| Requirement | This revision |
|---|---|
| 1. Directionality | Met by convention. The field marker states the role: a regular field is a function its definer provides, and a `!` or `?` field one its user must or may supply (section 8). The check needs no role, because a signature binds both the callers and the implementation of every function field (section 2). |
| 2. No spooky action at a distance | Met. A signature adds caller and implementation errors and never removes one. An implementation never widens calls. Supplying a missing implementation can complete a call that was incomplete, as adding information does anywhere in CUE. |
| 3. Input and output invariants | Met. Constraints on calls are unchanged. |
| 4. `f & f` is callable | Met. A function is the same however it is reached, through references, lets, instantiation, or copies of its struct, as long as what it captures is equal (section 6). |
| 5. FFI stays idiomatic | Met for Go `int`, `int64`, `uint`, and `uint64` by default, and for narrow integer kinds through an override at registration. Without an override, narrow kinds are exposed exactly (section 7). |
| 6. No extra power | Met. The checks compare declarations only and compute nothing a struct could not. Disallowing composition also removes the multiplied evaluation of nested calls. |

**Open questions.**

- Re-checking pending obligations once the values they refer to are complete.
- Syntax for the projections of section 9.
- Whether distinct implementations should combine later, and how (section 6).
- How a registration states an override, how native types, guards, and
  partiality are presented to tools, and whether the `@go` notation is adopted.
- The policy for body checks: defaults, flags, and what a certified interface
  may promise.

---

## Acknowledgments

Thanks to Aram Hăvărneanu, whose Quantified CUE counterproposal and paper showed
how standard contravariance can apply to CUE functions while meeting all but one
of the addendum's requirements, and supplied several of the principles this
revision adopts, and to everyone who discussed the functions proposal in
discussion 4484.
