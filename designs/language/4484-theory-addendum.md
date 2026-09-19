# Why CUE Functions Tighten Instead of Widen

## Variance in the CUE functions experiment

> **Status: experiment.** Functions are gated behind `@experiment(functions)`. Experiments exist to collect real usage before a design is fixed, and we are willing to adjust or retract one. When in doubt we ship the more controversial variant first, because it produces the sharpest feedback.

**Shipped as experiment.** As of CUE `v0.18.0` (currently alpha), the experiment ships covariant tightening for signatures and for bridged implementations (section 5), with checks at call time, and pointwise conjunction of bodies (5.2). The coverage obligation (6.2) and the compatibility tooling (6.3) are proposals. Code marked `cue` uses the experiment's syntax; `func!` in 6.2 is proposed syntax; arrows in plain fences are notation.

**In one paragraph.** In most languages a function that accepts *more* may replace one that accepts less: inputs are contravariant. CUE's `&` on functions does the opposite: `func(number) & func(int)` is `func(int)`. This is deliberate. The usual reason for contravariance is safe replacement, and for a function-valued field in CUE that question has no role-free answer: the field may be something a module provides or something a user must supply, and the two roles disagree about which change is safe. So a bare function field is *invariant under replacement*, and replacement cannot tell `&` which way to go. CUE's `&` already ignores role for data fields and simply tightens; functions follow suit. The signature is read as a constraint on calls, violating it is `_|_`, `&` on signatures is componentwise tightening, and the operation is symmetric and closed. This is a design choice, not the only coherent one, and it has a cost: a plain signature does not promise that an implementation covers its domain. Section 6 shows how that promise can be added, and where contravariance is genuinely needed, in checking that a new version can replace an old one, CUE uses it, in a tool rather than in `&`.

---

## 1. What functions are for

We have long been hesitant about adding functions to CUE. Based on extensive experience with configuration systems, we came to two seemingly contradictory conclusions:

1. **Code has no place in configuration.** Once you can write arbitrary logic, configurations stop being data and become programs. They become hard to compose, hard to analyze, and hard to trust.
2. **Practical applications require code in the configuration layer.** The realities of the real world mean that some computation cannot be offloaded to the software layers.

The way to reconcile these two halves is a strict separation: write code in a language made for code, alongside the configuration, and *bridge* it. This barrier forces a proper separation of concerns. We can then use CUE's lattice properties to compose the results in a well-defined manner. CUE's tooling layer (`_tool.cue`) grew out of exactly this idea and was included early on as a critical part of the proof of concept.

Given this history, it surprised some people that we added functions at all. So let us be explicit about the goals.

Functions in CUE therefore mostly exist to **describe** code that lives elsewhere (foreign functions and builtins, RPC interfaces in OpenAPI-style schemas, APIs in general) and to make it convenient to constrain their inputs and outputs. Simple bodies are allowed, but the design points users toward Go and toward the bridge, and "programming" in CUE is still discouraged. Several arguments below rest on this: **the main use of a CUE signature is to describe an implementation in another language.**

---

## 2. The standard rule, and the question it answers

Write `A ≤ B` for "every `A` is a `B`". A function `C → D` may safely stand in for `A → B` when it accepts everything the original accepted and returns something the caller can handle:

```
(C → D)  ≤  (A → B)     when    A ≤ C   and   D ≤ B
```

Inputs flip direction (**contravariant**), outputs keep it (**covariant**). Languages insist on this because it makes substitution safe: if a caller of `number → string` is handed an `int → string`, the call `f(1.5)` was declared fine and cannot be handled.

This rule answers one question: *may this function replace that one?* CUE asks that question too (6.3), and gives the standard answer. But `&` asks a different question: *what do these two descriptions say together?* The rest of the document is about why the two questions get different answers.

---

## 3. Three things that do not fit CUE

The intuition to import contravariance into `&` is strong. Three concrete cases show that it does not do what people expect in CUE.

### 3.1 Overloading through `&` does not work in CUE

`&` between two functions is not a feature we set out to add. It follows from functions being values: two files that both declare `f` unify, so `f & g` has to mean *something*, and the only question is what.

The familiar candidate comes from TypeScript, where an intersection of function types, schematically `(int → int) & (string → string)`, is read as a function that handles both, with the language realizing it as several overload signatures over *one* body. It is tempting to want the same from CUE's `&`, now with a body per clause. It cannot be had in CUE, and the part of it that cannot be had anywhere is selection.

**The rule that decides it.** Unification only ever refines: `x & y` is an instance of `x`, `x & y ⊑ x`. For a concrete value that leaves two outcomes, stay or fail: `2 & int` is `2` and `2 & 3` is `_|_`. Nothing added elsewhere with `&` can turn a concrete `2` into a `3`. (Defaults and comprehensions are the two constructs where the observed value can move for other reasons: a default is a presentation of a non-concrete disjunction, and a comprehension recomputes from values that may themselves be refined. A function call is neither; it is a value derived from the function and its arguments, and unifying the function must act on it as `&` acts on any value.) Applied to calls the rule reads `(f & g)(x) ⊑ f(x)` for every bound `x`, and this is the lattice test.

```cue
f: func(x: int | string) -> int: 1
g: func(x: int)          -> int: 2
```

On the overlap (integers, which both accept), overloading would *select* the more specific body: `(f & g)(3) = 2`. But `f(3) = 1` is concrete and `2 ⋢ 1`: a value that was `1` has become `2`, which is exactly what the rule forbids. Leaving the overlap as `1 | 2` fails too, since a disjunction sits *above* its arms. What passes is unifying the results, `1 & 2 = _|_`, or rejecting outright.

**The contract reading does not recover it.** One might hope that reading signatures as contracts, TypeScript-style, gives overloading back. It does not, for the same reason: on inputs both clauses accept, the two bodies must *agree*, so there is still no dispatch. Where the domains are disjoint there is nothing to agree on, and then the `_` convention does give dispatch by type: `f = 1` on integers and `g = 2` on strings conjoin to a function returning `1` for `3` and `2` for `"s"`. That is overloading for disjoint types. CUE does not get it, because the `_|_` convention (5.1) makes such a conjunction fail on every input; the `_|_` convention is what keeps signatures single arrows, and that is the trade. What changes is the shape of the signature. Under the contract reading the meet of `func(int) -> string` and `func(string) -> int` says "on ints a string, on strings an int", and no single arrow expresses that; the closest, `func(int | string) -> (string | int)`, would also allow an int to produce an int. So the meet is a *clause list*, one arrow per input region. Subsumption can simplify such a list by dropping a clause implied by another (`func(int) -> string` is implied by `func(number) -> string`), and the list collapses to one arrow when the results coincide. What subsumption may never do is pick which clause *answers a call*: that is selection by specificity, and it changes results. The examples people want intersections for are exactly the irreducible ones.

**Rejecting is not free either.** That leaves two options: reject two distinct bodies (`f & g = _|_` unless `g` *is* `f`), or unify their results. Rejecting looks conservative, since it adds no semantics. But it must still allow `f & f = f`, and that requires deciding whether two functions are *the same*, which a structural language does not otherwise define:

```cue
x: {f: func(y: int) -> int: y}
a: x.f & x.f            // same function: must be f
b: x.f & {x}.f          // same function reached by embedding: must be f, though the evaluator may hold a different object
c: x.f & (x & {g: 1}).f // same function after an unrelated unification: must be f
d: [for v in [1, 2] {func(y: int) -> int: y + v}]
e: d[0] & d[1]          // same source text, different captured v: must be _|_
```

`b` and `c` must succeed although the evaluator may see different objects; `e` must fail although the source is identical. A definition exists (same definition site, structurally equal captured environment), but it is a second equality, independent of `&`, that users must learn and tooling must preserve through embedding, selection, comprehension, and unification with signatures, and that compares arbitrary captured values at unification time. Unifying results asks none of this: `(f & f)(x) = f(x) & f(x) = f(x)` by idempotence, `b` and `c` are `f(y) & f(y)`, and `e` is `_|_` on the first input that exposes the different `v`.

**What remains is pointwise conjunction.** Select is out by the refinement rule, reject is a cost we choose not to pay, and unifying results needs nothing new: only results are ever compared, and CUE already knows how to compare values.

```cue
f: func(x: int) -> {...}: {a: x}
g: func(x: int) -> {...}: {b: x}
r1: (f & g)(3)        // {a: 3, b: 3}

h: func(x: int) -> int: 1
k: func(x: int) -> int: 2
r2: (h & k)(3)        // _|_: conflicting values 1 and 2
```

Both contributions survive. A conflict between bodies surfaces at the call that exposes it, not on the function value; how that deferral is presented in diagnostics is part of the design, since a struct with a conflicting field is an error outright.

*Implementation note.* Identity has no role in the semantics above, but it still has a place in the evaluator: where two operands are known to be the same function, the pointwise result is known without evaluating twice, so identity remains useful for caching and cycle detection. An imprecision there costs performance, not correctness.

**Pointwise conjunction is covariant on inputs.** This is not a separate decision; it follows from `(f & g)(x) = f(x) & g(x)` plus the rule that a call outside a body's parameter constraint is `_|_`. Return to `f` on `int | string` and `g` on `int`. On the overlap the results meet. Off the overlap, on a string, `g("s") = _|_`, and `_|_` absorbs, so `(f & g)("s") = _|_`. The declared domain of `f & g` is the *intersection* of the two declared domains; the calls that actually succeed may be fewer still, as `h & k` shows. With the opposite rule, where an out-of-domain call is `_`, the same computation gives `(f & g)("s") = f("s")`: the domain is the *union*.

That is what variance is, seen on the input position. Contravariance swaps the lattice operations there: `&` on functions acts as `|` on domains, and `|` on functions as `&` on domains. Covariance leaves them as they are. So pointwise conjunction under the `_|_` rule does on inputs exactly what `|` on functions would do in the contravariant model, and the result position, which flips in neither model, is the same in both. The equation itself is neutral; the variance comes from what an out-of-domain call evaluates to, which is the premise of 5.1. What the equation adds is that bodies and signatures cannot go different ways: whichever rule is chosen for one applies to the other, or the associativity failure of 3.3 appears. This design takes the rule to be `_|_` (5.1), so both are covariant.

### 3.2 The contract reading's early check does little for FFI

Under the contract reading, `func(number) -> number` promises that the implementation's *declared* domain covers every number, and the promise is checked when a body is attached. The appeal is that mismatches are caught early, without seeing call sites. What does that buy for the main use case, describing a Go function?

Every FFI binding goes through a bridge, needed under either variance: apply the CUE parameter constraints (wait if the argument is not concrete), check representability, convert (exactly for integers; floating-point conversion rounds to the nearest representable value), call, convert back, apply the result constraint. So the question is what the early check catches that the bridge does not.

Take Go's `func f(a int) int`. CUE's `int` is arbitrary precision; Go's is 32 or 64 bits.

```cue
f: func(x: int) -> int          // idiomatic: 10^30 fails at the bridge
f: func(x: int64) -> int64      // native bound in the public signature
```

Whether the early check catches the native limit depends on what the signature describes. If it describes the *checked adapter*, `int` is honest under either reading, the check passes, and `f(10^30)` fails at the bridge as a permitted runtime failure. If it describes the *raw native function*, the honest signature is the second one, and the limit is caught early at the price of target-dependent detail in every public signature. The same fork recurs for several basic types: `float64` versus arbitrary-precision decimals, Go byte strings versus CUE Unicode strings (a Go function may *return* invalid UTF-8), homogeneous slices versus CUE lists.

What the early check does catch under the idiomatic signature is a declaration that does not fit the native function. The gross case is a kind mismatch, a CUE `string` parameter against a Go `int`, which is caught anyway when a signature is unified with a builtin. The subtler case has overlapping domains: `func(x: number)` against a Go `int` is neither a kind mismatch nor a width limit, tightening succeeds, and only a fractional argument exposes it, at the call. A coverage check rejects that declaration without waiting for the call, and this is a real advantage. But it is not one that requires the contract reading, because the FFI boundary is the one place where the role is never in doubt: the Go function is the implementation and never a restriction. So the bridge can make the same directional comparison at registration, the CUE parameter constraint against the Go type's natural constraint, `number ⊑ int`, and reject the declaration there. That check is contravariant, and it is legitimate precisely because at the boundary directionality is known (section 4). For bridges, then, the early check either duplicates what registration already does, or catches native limits only when they have been written into the signature. That is not a reason to change the meaning of `&`.

Our preference is an engineering one and is available under either variance: the public signature states the portable intent (`int`), the bridge enforces the native bound at the call, and the bound stays available to diagnostics and tooling as a hidden constraint:

```
F(42)                                → 42
F(1000000000000000000000000000000)  → _|_: argument does not fit Go int
```

The effective constraint is `int & GoInt`, both applied when a concrete argument arrives. What covariance adds is that the bridge does not have to pretend: it can say that a signature restricts calls and that the adapter restricts them further, which is exactly what happens.

### 3.3 Under the annotation-style contract reading a signature does not constrain calls

```cue
// schema.cue (platform team)
lookup: func(x: int) -> string           // only integers may be looked up

// impl.cue (a Go binding, or a user)
lookup: func(x: number) -> string: "\(x)"

y: lookup(1.5)
```

Under the contract reading, in the form where attaching a contract leaves the implementation unchanged, the binding succeeds (`number` covers `int`) and the body is used as is, so `y` is `"1.5"`. The platform's `int` constrained nobody. A contract binds whoever supplies the body, and no one else. (Runtime contract systems such as Racket's instead wrap the implementation in checks; in CUE that wrapping is what tightening does, and what the bridge does at the FFI boundary.) But what CUE users compose across files and teams are constraints on *data*, and arguments are data. A schema author who writes `port: int` and `lookup: func(x: int) -> string` expects both `int`s to bind the people supplying values; under the contract reading only the first does. Under covariance `lookup` is the body restricted to `int` and `y` is `_|_`.

Many people who ask for contravariance actually picture a hybrid, and there are two of them. They fail for different reasons.

**Hybrid A: signatures merge contravariantly, and the merged signature restricts calls.** Take the effective parameter domain to be the union of the declared ones and the result constraints to meet, and use the merged signature as the guard on calls. Keep the result fixed so that only the domain is in play:

```cue
// file 1
f: func(x: int) -> int: 1
y: f(1.5)                        // _|_

// file 2
f: func(x: number) -> int        // hybrid A merges to func(x: number) -> int, and y becomes 1
```

A constraint in a file you cannot see has un-failed a call in a file you can, which is exactly what CUE's error monotonicity exists to prevent. This rules out the union-of-domains rule as a guard on calls.

It is worth being precise about what covariance rules out here, because the second file's author may well *intend* to widen. The most permissive thing they can write is a function that accepts every number and constrains nothing:

```cue
// file 2, trying to widen
f: func(x: number) -> _: _
```

Under pointwise conjunction `(f & w)(1.5) = f(1.5) & _ = _|_ & _ = _|_`. Even a body that returns top cannot un-fail the call, because `_|_` absorbs. This is the same fact as `x: int` followed by `x: number` staying `int`: in CUE no file can widen what another file constrained, and functions are not an exception. Widening is something a *new version* does to an *old* one (6.3), never something one conjunct does to another.

There is still action at a distance here, seen from file 2: its author never saw file 1's `int`, and it makes their own `f(1.5)` fail. But that is the direction CUE permits for every field. A constraint you cannot see can add an error to your code; it can never remove one. Hybrid A violates the second half, and that is the whole objection to it.

A fully contravariant design avoids this, but note how: the difference is *where the error goes*. Hybrid A merges file 2 into `f`'s guard: `f` now accepts numbers, `y` is `1`, and the error that was in file 1 has silently vanished. A statically typed language does the opposite. File 2 is not a new `f`; it is a claim *about* `f` ("`f` handles every number"). The checker compares that claim with the body, finds it false, and the error lands in file 2, while `y` stays `_|_` because `f` never changed. In CUE that is the `R ⊑ D` design of 6.2: attached signatures are claims, the body's declared domain is what they are checked against, and nothing is merged into the guard.

**Hybrid B: signatures merge covariantly, but attaching an implementation is checked contravariantly.** With `I = func(int) -> string`, `N = func(number) -> string`, and `b` a builtin accepting only integers: covariant merging gives `I & N = I`; contravariant checking gives `b & I = b` and `b & N = _|_`. Then `b & (I & N) = b` but `(b & N) & I = _|_`. Associativity is gone. The cause is that `I & N` *dropped* `N`'s obligation, because a tightening constraint only narrows. This rules out mixing the two directions inside one `&`. It does not rule out a consistently contravariant design (`I & N = N`, both groupings reject `b`), nor a design that keeps the obligation as separate information (6.2), which is where the obligation-as-marker idea comes from.

---

## 4. Function fields are invariant under replacement

The deeper reason no contravariant rule feels right is that the *field* holding a function does not have a direction of its own.

```cue
old: {handler: func(x: int)    -> string}
new: {handler: func(x: number) -> string}     // the parameter was widened
```

Is this edit compatible? It depends on something the value does not say: who provides `handler`.

| Who provides `handler` | The edit means | Existing users | Compatible? |
|---|---|---|---|
| The **module** provides it; users call it | users may now pass more | their `handler(3)` still works | yes |
| **Users** must supply it; the module calls it | users must now supply a function that handles more | a user's `func(x: int)` no longer covers the requirement | no (under the contract reading) |

Same edit, same file, opposite verdicts. Narrowing runs the other way.

This is the situation type theory knows from mutable references: reading is covariant, writing is contravariant, and a cell that is both read and written admits no non-trivial subtyping, which is what **invariant** means. A CUE field behaves like such a cell. The module may call the function (read) and a user may supply it (write), and nothing in `{handler: func(x: number) -> string}` records which. So a replacement that must be safe without knowing the role must be compatible in *both* directions: a bare function field is invariant under replacement.

This is a fact about replacement, and it needs stating carefully, because it decides less than it seems to. Data fields have exactly the same role-dependence: widening `{port: int}` to `{port: number}` is compatible if the module accepts `port` and breaking if it produces it. Nobody concludes from that that `int ⊑ number` is in doubt or that `int & number` is undetermined. Compatibility for data fields depends on role; `&` on data fields ignores role and tightens; the two facts coexist without tension.

More importantly, the two readings behave very differently in practice. Under tightening, `func(int) & func(number)` always succeeds: it is `func(int)`. The domain has narrowed, as it does for any constrained value, and a caller who relied on the wider domain now fails at their call with `1.5`; nothing else changes, and nothing has gone wrong, since that is what the constraint means. Under the contract reading, attaching an `int` body to `func(number)` fails at binding, whether or not anyone will ever pass a non-integer. If the wider signature was a restriction on callers, that failure is spurious; if it was a coverage requirement, it is correct. So the contract `&` decides at binding time what the role was, and gets it wrong for one of the two roles; the covariant `&` makes no such decision and leaves it to the calls that actually occur. Both operations are symmetric; only one is role-independent in what it does. Contravariance is therefore usable only where the role has been made explicit, and 6.2 makes it explicit by making coverage opt-in.

That settles the direction. A `func` signature is a constraint on values: the argument must satisfy the parameter constraint whoever passes it, the result must satisfy the result constraint whoever produces it, which is what `&` does to every other field and what a schema author means by `func(x: int)` (3.3). What such a signature does not say is whether the body *covers* its domain. That is the one thing the contract reading adds, and it is an obligation on an implementation rather than a constraint on a value, so it cannot be had by narrowing anything. 6.2 gives it its own declaration form, `func!`, which unifies like everything else and accumulates the obligation instead of reversing `func`.

---

## 5. Covariance: a reading consistent with CUE

### 5.1 Where CUE's order and standard subtyping differ

In CUE, types and values share one lattice, `5 ⊑ int ⊑ number ⊑ _`, with `&` as meet. A signature follows the same principle given one premise:

> A signature is a constraint on **calls**: the argument must satisfy the parameter constraint and the result must satisfy the result constraint. It does not by itself promise that an implementation handles every value in the parameter constraint.

This is a choice; the lattice does not force it. What the choice decides is what a call *outside* the declared domain evaluates to, and that one detail is the whole difference between CUE's order on signatures and standard function subtyping. Treat a signature as a map over concrete, fully bound arguments, abstracting what it constrains rather than what a body computes:

```
S(A, B)(x)  =  B      if x is in A
            =  ?      otherwise
```

Order such maps pointwise and fill in the `?` both ways. For non-empty `A`, `A'` and `B`, `B'` neither `_|_` nor `_` (the degenerate cases collapse, `S(int, _|_) = S(string, _|_)`, and `S(∅, string)` is constantly `_|_` whatever its result, so the converses fail there):

- **`?` is `_|_`** (a call outside the domain is invalid): `S(A,B) ⊑ S(A',B')` iff `A ⊑ A'` and `B ⊑ B'`. **Covariant.**
- **`?` is `_`** (a call outside the domain is unconstrained): `S(A,B) ⊑ S(A',B')` iff `A' ⊑ A` and `B ⊑ B'`. **Contravariant.**

The second line is standard function subtyping, derived rather than assumed. So the two orders disagree by one convention: whether a type says nothing about calls it does not cover, or forbids them. CUE takes the first bullet, for the reasons in section 4.

The first bullet has one practical consequence worth naming. The pointwise meet of two signature maps is again a single signature map, componentwise, degenerate cases included:

```
S(A,B) & S(A',B')  =  S(A & A', B & B')
```

Under the second bullet the meet covers `A ∪ A'` with different results on different parts, a clause list (3.1) rather than a signature. Single-arrow signatures are **closed under `&`** only under the first bullet, so a tool reading two signatures can write down their conjunction without evaluating anything.

### 5.2 Three operations

In most languages the relation on function types that matters is subtyping, and it serves one purpose. In CUE there are three operations on functions, all written `&`:

| Operation | Example |
|---|---|
| Signature `&` signature | `func(int) -> string & func(>0) -> string` |
| Implementation `&` signature | `strings.ToUpper & func(string) -> string` |
| Implementation `&` implementation | `f & g` for two bodies |

Intersection-type systems (TypeScript, Scala 3, MLsub) have the first. None has CUE's other feature: a type and a value are the same kind of thing, so all three are one operation. Pointwise conjunction, for calls whose arguments are fully bound against the unified parameter list, makes them so:

```
(f & g)(x)  =  f(x) & g(x)
```

A bodiless signature contributes `S(A, B)`, so `f & S` is `f(x) & S(x)`: the argument meets the parameter constraint, the result meets the result constraint. This means a signature is **not an annotation**. In an ordinary language `f : number → number` describes `f` and changes nothing; in CUE `f & S` is a new function value:

```cue
f: func(x: number) -> number: x + 1
g: f & func(x: int & >=0) -> int

a: g(2)     // 3
b: g(0.5)   // _|_: 0.5 is not an int
c: g(-1)    // _|_: -1 is not >= 0
```

`g` is not a drop-in replacement for `f`; some calls that worked for `f` fail for `g`. That is fine. `x: number, x: int` rejects `1.5`, and nobody calls that a bug. Tightening a signature is refinement applied to calls.

Two further properties follow, **for pure bodies on fully bound arguments**. The lattice laws (commutativity, associativity, idempotence) are inherited from the result lattice, with no appeal to declaration order or specificity. And no notion of function *identity* is needed, since only results are ever compared (3.1). Note also that the variance of `&` on bodies and on signatures is not two choices but one: pointwise conjunction under the `_|_` rule narrows the declared domain of `f & g` to the intersection of the two (3.1), which is exactly `S(A & A', B & B')` from 5.1.

The scope matters. The equation is stated over fully bound calls because binding is part of the operation and does not follow from result unification:

```cue
f: func(x: int) -> int: x
g: f & func(x: int = 2) -> int
r: f()     // _|_: missing argument
s: g()     // 2
```

The equation holds once `x` is bound to `2` on both sides; it cannot be read as "the same raw call on both operands". Still to settle: binding when signatures differ in labels or arity (parameter lists should unify like structs); explicit defaults (two different defaults conflict); captured scopes (each body evaluates in its own environment, though the implementation shares the matched argument constraints and defaults among the body activations, so "only results meet" is the idealization); and effects, for bridged implementations only, since CUE bodies have none (section 7). Statements about errors never being repaired also need to distinguish a definite constraint conflict from a call that is merely incomplete.

### 5.3 No new failure mode

The classic objection to covariant inputs is that a violated parameter type leaves the program without the guarantee the static check promised, and in some languages that shows up as a type confusion or a crash. In CUE the parameter constraint is checked at the call, and a violation is `_|_`: an ordinary value that propagates monotonically, is reported with a location, and can be inspected. Covariant inputs cost CUE no failure mode it does not already have. This says nothing about failures *inside* a native body: a panic or an overflow that wraps in Go is invisible to the bridge under either variance. Dart is the closest precedent: an overriding method may narrow a parameter with the `covariant` modifier, and a call through the wider interface throws at runtime. CUE's error is a value rather than an exception; that is a difference in representation, not a claim of extra safety.

---

## 6. What covariance costs, and how to pay for it

### 6.1 The cost: modular checkability

```cue
apply: func(f: func(number) -> number) -> number: f(1.5)
all:   func(x: number) -> number: x
ints:  func(x: int)    -> number: x

ok:  apply(all)   // 1.5
bad: apply(ints)  // _|_: conflicting values 1.5 and int
```

`ints` was accepted where `func(number) -> number` was expected, and the problem surfaced at the call. A contravariant checker would have rejected it at binding, without seeing call sites. That property is real and plain tightening does not have it. It is worth keeping the loss in proportion, though. Function values in CUE are computed at evaluation time anyway, so there is no separate compile step at which an early rejection would be cheaper than a late one. The bridge, which is needed regardless of variance (3.2), already handles errors at the boundary and can report a domain mismatch with the same precision. And tooling (6.3) can still perform the directional check where it matters, outside `&`.

### 6.2 Coverage as a declared obligation (proposal)

The check cannot be recovered from the parameter constraint, because tightening only narrows: `func(int) & func(number)` is `func(int)`, and whatever `number` was meant to promise has been unified away. An obligation to *cover* a domain moves in the opposite direction from a constraint, so it must be carried separately. Required fields are the precedent:

```
{a!: int} & {a: 3}   =  {a: 3}
{a!: int} & {b: 1}   =  {a!: int, b: 1}
```

The obligation rides along in the value; `&` accumulates it without knowing whether it is met; a separate completeness check reports it. The analogy supports carrying an obligation; it does not by itself prove that coverage is met.

Do the same for parameters. A parameter carries a call constraint `C` (today's meaning) and optionally a required domain `R`. Under `&`, `C` meets, `R` joins, bodies combine pointwise. The consistency check is `R ⊑ C`, with three outcomes: established, in which case the value stands; refuted, in which case the value is `_|_`; and *cannot establish*, when the constraints reference values not yet resolved, in which case the value is incomplete rather than `_|_`. For fixed domains the algebra is well behaved: componentwise meets and joins are associative, commutative, and idempotent; `R` only grows and `C` only shrinks, so a refuted check cannot be repaired by further unification. Two possible surfaces:

```cue
// func! sets both C and R for its parameters; func sets only C
f: func!(x: number) -> string
g: f & func(x: int) -> string     // _|_: obliged to cover number, restricted to int

apply: func(f: func!(x: number) -> number) -> number: f(1.5)
bad:   apply(ints)                // _|_ at binding
```

`func!` reads as the required-field precedent it is modeled on: `a!: int` says someone must still supply `a`, `func!(x: number)` says someone must still supply an implementation that covers `number`. Results need no required domain since they are covariant under either reading. A `func!` with a body is meaningful too: a provided implementation that also promises coverage, which is the bidirectional case of 6.3.

`func!` also gives the compatibility tool the one bit it can read off a declaration: that someone other than the author may supply the body and must cover the domain (6.3). It does not say who calls, and a body next to `func!` is not a default that users override: a second body conjoins with the first under pointwise conjunction, it does not replace it, and both must cover `R`. Coverage and role stay separate properties; `func!` is a coverage obligation that happens to imply one half of the role.

The associativity failure of hybrid B in 3.3 disappears because `I & N` keeps `N`'s obligation in `R`. Note what the check establishes: declared-domain consistency, not success. A Go binding whose native bound is outside `C` passes and fails on large inputs. That is the strength conventional contravariant typing has, no more.

It is also input-side only. `impl: func(x: number) -> number: x` unified with `func!(x: number) -> int` passes `R ⊑ C`, and `f(1.5)` then fails its result constraint; conventional checking would have rejected the attachment, since the body's declared result `number` is not within `int`. Recovering that needs a second check at attachment, that the body's declared result is subsumed by the attached result constraint, which is a subsumption between two values, not a meet. It is a natural extension and is listed as open.

**An alternative: every bodiless signature is a promise.** `func!` makes coverage opt-in. The opposite default is to treat every bodiless signature as a coverage requirement: bodiless signatures contribute `R`, a body contributes its declared domain `D`, and the check is `R ⊑ D`. Specialization survives (`f` declared on `number`, attached `func(int)`: `int ⊑ number`), `apply(ints)` fails at binding, and the check is associative. But this is the contract reading with the role *guessed*: "no body" is taken to mean "someone else supplies this and must cover it", the body-or-hole proxy of 6.3 made into semantics. In effect bodiless signatures stop being value constraints that unify and become a set of contracts each implementation must satisfy; what a schema author uses a signature for (3.3) is what this design gives up. Section 4 says what goes wrong: whenever a bodiless signature was a restriction on callers, `handler: func(x: >=0)` against a user's `func(x: int)`, the binding fails spuriously. Nor does it help compatibility: the tool still has to ask who provides `f`, and this design answers with the same proxy, so where the proxy is wrong, a Go binding or a call site outside CUE, only static analysis or an explicit role would fix it. Contravariance is usable only where the role is explicit; this design assumes it instead, which is why `func!` is preferred.

**Why not check at the call instead?** A natural alternative keeps `&` as pure tightening and, at each call, checks the implementation contravariantly against every signature attached to it. This looks like Hybrid B, but differs in the one respect that matters: Hybrid B merges signatures first and loses `N`'s obligation in `I & N = I`, whereas a call-time check must keep every attached signature in order to check against it. That list is `R` under another name, it accumulates as a set, and associativity is safe. It does not, however, escape the role question: a single argument can be checked against `C`, which tightening already does, but coverage is a claim about a whole domain, so the check must still decide which attached signatures are promises. It is therefore not a third design but one of the two above with the check postponed. If every attached signature counts, it is "every bodiless signature is a promise", role guessed and all, with the same spurious failure on a bodiless restriction (`handler: func(x: >=0)` against a user's `func(x: int)`). If only marked signatures count, it is `func!`, and postponing the check gains nothing, since the marked signatures are known at binding, and costs locality: inside `apply(ints)`, the call `f(2)` would fail although `2` is an integer, with the error at a call that would have succeeded rather than at the attachment that caused it.

### 6.3 Backward compatibility: where CUE uses contravariance (proposal)

Whether `new` can replace `old` for existing callers is the question of section 2, and the standard answer applies. This is a job for tooling, not the language: it compares two published versions, never two conjuncts, and would live in a future compatibility checker run at module publication or in CI.

```
compat(old, new):
  parameters:   old.C ⊑ new.C for each pair              // accepts at least as much
                pair by position for positional, by label for named
                new may add parameters only if they are omittable
                new may not remove, rename, or make positional-only
                  parameters callers can name, nor make named-only
                  parameters callers pass positionally
                new may not remove a default or make an omittable
                  parameter required
  result:       new.C ⊑ old.C                             // promises at least as much
  required domains (6.2):  old.R ⊑ new.R
  callbacks:    compat(new.param, old.param)               // the higher-order flip, using compat itself
  dependent or relational constraints: cannot establish
```

The call-shape rules matter as much as the inclusion checks. Changing `func(x: int = 2) -> int` to `func(x: int) -> int` leaves every inclusion check unchanged and breaks every existing zero-argument call; that is a signature-level break, not a behavior change.

There is no tension with section 5. Combining two descriptions and replacing one with another are different questions, and CUE already has this split for plain schemas: data a module *accepts* may widen, data it *produces* may narrow. A signature bundles both roles.

**Who calls, and who supplies.** The rules above are for a function the module provides and users call. For one users supply and the module calls, the parameter direction flips: widening is breaking, since users must now cover more. The declaration says one of these two things and not the other. `func!` says users must supply and cover, so widening a `func!` parameter is breaking. Nothing says whether users call: a bodiless `func` is a hole users fill, but the user who fills it may also call it, so its accepted domain is something callers may depend on. The tool therefore assumes what it already assumes for data, that anything exported may be used, and treats narrowing a parameter as always breaking. That leaves one rule: **widening a parameter is breaking iff the field is `func!`; narrowing is always breaking.** Put differently, a `func!` field is invariant in both parameters and result unless the module declares that it alone calls it; a plain `func` field may widen its parameters and narrow its result. An explicit role supplied to the tool can relax the rule where the module knows nobody else calls.

Compatibility cannot be `⊑`. With `old: func(x: int) -> number` and `new: func(x: number) -> int`, `new` is compatible with `old`, neither subsumes the other, and `old & new = func(x: int) -> int` describes neither. Subsumption compares two values; compatibility compares two values *in a role*, and the role is not in the values.

| | Compares | When | Needs an orientation between operands? |
|---|---|---|---|
| `R` check (6.2) | a single value's own `R` against its own `C` | during unification | no, hence safe inside `&` |
| Compatibility (6.3) | `old` against `new` | in a future compatibility checker | yes, hence outside `&` |

Both use `⊑`, which is directional; what differs is whether `&` must decide which operand is which. `compat` inherits the three outcomes of subsumption: compatible, incompatible, cannot establish. In practice `Subsume` reports an error rather than a three-valued verdict and tracks the possibility of false negatives, so a tool has to decide how to classify its failures rather than assume an exact oracle. The check does not cover behavior, native representation limits unless supplied as hidden constraints, or actual as opposed to declared coverage.

---

## 7. Execution model: no more power than structs

Up to now, CUE users have written "functions" as structs: the parameters are fields, the body is a field computed from them, and a call is a unification followed by a selection.

```cue
#Add: {
    x:      int
    y:      int
    result: x + y
}
sum: (#Add & {x: 1, y: 2}).result    // 3
```

The function form says the same thing:

```cue
add: func(x: int, y: int) -> int: x + y
sum: add(1, 2)                        // 3
```

The correspondence is exact by design. A function body evaluates under the same rules and the same cycle restrictions as a struct, with the parameters playing the role of the sibling fields. Whatever a call can express, the struct pattern could already express. Functions make it nicer to read and write, easier to constrain (section 1), and solve some awkwardness with defaults. But they are deliberately not an enabler of more computation in CUE.

One consequence deserves spelling out. Because a body evaluates as a struct does, recursion in CUE functions is bounded by the same rule that bounds structs: a call that would re-enter itself without new information is a structural cycle, detected and rejected exactly as it is for structs. There is no general recursion, and hence no way to compute more than a struct could: you cannot write Ackermann as a CUE function, or Fibonacci for that matter.

This is orthogonal to the variance question: nothing in sections 3 to 6 depends on what a body may do. It does dispose of one item. Pointwise conjunction (5.2) evaluates both bodies for one call, and whether that has consequences is listed there as open. For bodies written in CUE it does not: two bodies evaluated for one call are no more remarkable than `#Add & #Mul & {x: 1, y: 2}`, each conjunct in its own scope, results unified. The question remains only for bridged implementations, whose effects the struct model does not have.

---

## 8. Summary

| Question | Answer | Why |
|---|---|---|
| Can `&` give overloading? | Not by specificity; disjoint dispatch only under the `_` convention | Selecting a body would change a concrete result, violating `(f & g)(x) ⊑ f(x)`; the contract reading gives agreement and clause lists, not dispatch; rejecting distinct bodies needs function identity, a cost we decline; unifying results needs nothing new and is covariant on inputs given the `_|_` rule (3.1) |
| Does the contract reading's early check help FFI? | Only marginally | Declarations that do not fit the native function, including overlapping domains such as `number` against Go `int`, are caught at registration, where the role is known; native limits only when they are written into the signature (3.2) |
| Does an annotation-style contract signature constrain calls? | No | It binds implementors, not callers; both hybrids that try to have it both ways fail, one on monotonicity, one on associativity (3.3) |
| What is the variance of a function field under replacement? | Invariant | Provided or consumed, and the value does not say which; the same holds for data fields and does not decide `&` (4) |
| So what does `&` do? | Covariant tightening, `(f & g)(x) = f(x) & g(x)` | Substitutability cannot orient `&` without a role; reading a signature as a constraint on calls gives a symmetric, closed, identity-free operation; a design choice, not the only coherent one (4, 5) |
| What is lost? | Modular checkability | Possibly recoverable by a declared, opt-in obligation, `func!`, like `!` on fields; a proposal (6.2). Making every bodiless signature a promise instead would assume the role section 4 says must be explicit |
| Where is contravariance used? | Backward compatibility | Replacement is a directional question with a role as input, so a tool, not `&` (6.3) |

A signature in CUE is read as a constraint on calls: a call outside the declared domain is `_|_`. Given that reading, the order on signatures is covariant, signatures are closed under `&`, and `&` on functions is ordinary tightening, role-blind like `&` on every other field. The reading is a choice; the alternatives are coherent and cost either the ability of a signature to bind callers or a clause list in place of a signature. The price of our choice is that a plain signature does not promise coverage, and that promise can be added explicitly rather than by changing what `&` means.

**Open questions.**

- Syntax for the coverage obligation; `func!` is the current proposal, and a more explicit spelling may be wanted (6.2).
- Should unequal signatures simply conflict, so that `func(int) & func(number)` is `_|_`? (Invariance under replacement does not by itself imply this; incomparable values need not have `_|_` as their meet.) If so, how are implementations attached, and how does specialization (5.2) work?
- It seems like it should be possible to have *calls*, rather than `&`, perform the variance check, but nothing simple has emerged; 6.2 lists the obstacles found so far. Open to suggestions.
- Interaction of `R` with defaults and omittable parameters.
- Whether `func!` should also check the body's declared result against the attached result constraint at binding (6.2).
- Whether native bounds may enter `C`.
- Effects of bridged implementations, and partial applications, under pointwise conjunction (5.2, 7).
- How much of the FFI surface wants coverage obligations.

---

## 9. Requirements for alternatives

We are open to any proposal that improves on this design, provided it meets the requirements the current design meets. In rough order of importance:

1. **Directionality.** It must work without knowing whether a function field is provided or consumed, or it must make that role explicit in the value (section 4). Guessing the role is not acceptable.
2. **No spooky action at a distance.** A constraint in one file may add an error to another file's evaluation; it may never remove one, and it may never change a concrete result to a different concrete result in code that locally does not use defaults or comprehensions (3.1, 3.3).
3. **Input and output invariants.** It must remain possible to constrain a function's inputs and outputs across files, though not necessarily through the signature itself (3.3).
4. **`f & f` is callable.** Unifying a function with itself, however the two occurrences were reached, must yield a callable function, without relying on pointer identity (3.1).
5. **FFI stays idiomatic.** Describing a Go function must not require native representation limits in the public signature, and must not make the bridge more complex than it already is (3.2).
6. **No extra power.** Functions must compute nothing a struct could not (section 7).

A proposal that relaxes one of these should say which, and why the trade is worth it.

---

## Glossary

- **`⊑`.** `a ⊑ b` means `a` is an instance of `b`, equivalently `a & b = a`. Example: `5 ⊑ int ⊑ number`. Approximated by `Subsume`, which may fail to establish an inclusion that holds.
- **`_` (top), `_|_` (bottom).** Top has no constraints; bottom is the error value, an instance of everything. The orientation matches set inclusion of instances, `∅ ⊆ {1} ⊆ ℤ` as `_|_ ⊑ 1 ⊑ int`. It is the dual of the *information order* used in domain theory, where `⊥` carries no information, more specific means higher, and unification is a join. CUE adopts the convention common in graph unification for NLP, its roots: "no information" at the top, more specific lower, unification a meet.
- **Unification (`&`).** The most general value satisfying both operands; the lattice meet.
- **Covariant / contravariant / invariant.** A position that preserves the order, reverses it, or admits no non-trivial substitution in either direction. Function inputs are contravariant in standard subtyping; mutable cells are invariant.
- **Constraint reading / contract reading.** A signature restricts calls, versus a signature promises that an implementation covers its declared domain (5.1 vs 6.2).
- **Modular checkability.** Verifying that an implementation satisfies a signature without seeing call sites.
- **Pointwise.** Defined argument by argument: `(f & g)(x) = f(x) & g(x)`, for fully bound `x`.
- **`C` / `R` / `D`.** A parameter's call constraint (meets under `&`), its required domain (joins under `&`), and, in the alternative of 6.2, a body's declared capacity. Consistency is `R ⊑ C` (marker design) or `R ⊑ D` (promise-by-default design), each with a *cannot establish* outcome.
- **Bridge / FFI.** The adapter that calls a function implemented in another language, typically Go, converting and checking at the boundary.
