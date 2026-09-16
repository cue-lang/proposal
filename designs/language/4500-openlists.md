# Proposal: open lists and closed literals

2026/09/16

*   **Status**: Draft
*   **Lifecycle**: Proposed
*   **Author(s)**: mpvl@
*   **Relevant Links**: https://cuelang.org/issue/1999; experimental
    implementation in `cue-lang/cue`, enabled per file with
    `@experiment(openlists)`
*   **Discussion Channel**: https://github.com/cue-lang/cue/discussions/4500

### Objective

This proposal makes lists behave like structs with respect to closedness.
A list literal outside a definition becomes open data: it merges with other
lists element by element, as a struct literal merges field by field. Lists
are closed where structs are closed: inside definitions, and where the
author says so.

To say so, the proposal adds closed literals: `#[...]` for a list and
`#{...}` for a struct. Each closes one level. `#[1, 2]` means exactly what
the list `[1, 2]` means today.

Goals:

*   **Consistency**: one closedness model for lists and structs.
*   **Round-trippable data**: exporting a configuration and unifying the
    result with its source must not fail because a data list was closed.
    This is a small part of a larger effort to make `export(A) & A` always
    succeed.
*   **Explicit, local closedness**: a notation that states the closedness of
    one value where it is written, usable both by authors and by tools that
    print evaluated values.
*   **A mechanical migration**: existing configurations keep their meaning
    after an automatic rewrite.

### Background

Today a list literal is closed unless it ends in `...`, while a struct
literal is open unless it is a definition or passed to `close`. The
asymmetry shows up as soon as data is combined:

```cue
y: [3]
y: [_, 4] // error today: incompatible list lengths (1 and 2)
```

The struct equivalent, `y: {a: 3}` and `y: {b: 4}`, is fine.

The asymmetry also breaks round trips. Given

```cue
a: [...int]
x: [1]
y: a & x
```

`cue export` produces `a: []`, `x: [1]`, `y: [1]`. Unifying that output
with the source closes `a` to the empty list, which then conflicts with
`y`. Structs have no such problem.

Closedness itself has two further rough edges that this proposal
addresses:

*   **`close()` is a builtin.** Its argument is evaluated as a builtin
    argument, which does not carry the recursive closedness a definition
    gives its contents. In `#S: close({b: {a: int}})`, `#S.b` accepts
    unknown fields, although `#T: {b: {a: int}}` rejects them in `#T.b`.
*   **Closedness belongs to values, not names.** After evaluation, whether
    a value is closed follows from how its parts were unified, and a value
    can be closed while its children are not. A tool that prints evaluated
    CUE has no local way to state "this value is closed at this level" other
    than wrapping it in `close(...)`, and no way at all for lists, which are
    closed implicitly.

### Overview

Under the experiment:

| Program | Today | Proposed |
|---------|-------|----------|
| `y: [3]`, `y: [_, 4]` | error | `[3, 4]` |
| `e: []`, `e: [1]` | error | `[1]` |
| `x: [{a: 1}]`, `x: [{b: 2}, {c: 3}]` | error | `[{a: 1, b: 2}, {c: 3}]` |
| `#D: [1, 2]`, `#D & [1, 2, 3]` | error | error: index 2 not allowed |
| `#D: [...int]`, `#D & [1, 2]` | `[1, 2]` | `[1, 2]` |
| `x: #[1, 2]`, `x: [_, _, 3]` | (not valid) | error: incompatible list lengths |
| `#[{a: 1}] & [{a: 1, b: 2}]` | (not valid) | `[{a: 1, b: 2}]` |
| `#{a: 1} & {b: 2}` | (not valid) | error: `b` not allowed |
| `len([1, 2])`, `[1, 2] == [1, 2]` | `2`, `true` | `2`, `true` |

### Detailed Design

#### Open lists

A list is open or closed exactly as a struct with integer labels would be:

*   A list literal outside a definition is open. It may gain elements by
    unification; lists of different lengths merge position by position.
*   A list inside a definition is closed, recursively, like the definition's
    structs: an element beyond its length is not allowed, and struct
    elements reject unknown fields. Referencing the definition carries its
    closedness, as for structs.
*   A trailing `...T` keeps a list open and constrains further elements, in
    a definition or not. As for `{...}` in structs, a list with an ellipsis
    unified with another list within the same definition leaves the result
    open.
*   `close()` accepts lists and closes one level.
*   Lists produced by builtins, such as `list.Concat`, are data and hence
    open, unless the file that calls them does not enable the experiment.

Observing a list does not depend on whether it may grow: `len`, `==`, and
the Go API's `Value.Len` use the elements present, and a list is printed
with `...` only if it has an ellipsis. A data list `[1, 2]` therefore
exports, compares, and measures as it does today.

#### Closed literals

`#[...]` is a list literal closed as a list literal is today. It limits the
length of the list at its own level: a longer list is reported as
incompatible list lengths, an index past its end is an error, and its
length is final. Its elements are not closed by it:

```cue
@experiment(openlists)

a: #[{name: "x"}]
a: [{name: "x", port: 80}]     // fine: the element is open
a: [{name: "x"}, {name: "y"}]  // error: incompatible list lengths (1 and 2)
```

A closed list literal cannot end in an ellipsis.

`#{...}` is a struct literal closed one level, as `close({...})` is today: a
field not declared in it is not allowed, a trailing `...` opens it, and
embeddings contribute to it. Unlike the argument of `close`, its fields keep
the closedness of an enclosing definition.

Both literals may appear inside definitions, where `#[...]` is redundant,
and both may contain comprehensions. Two values that differ only in the
prefix are distinct alternatives of a disjunction.

#### Syntax

```ebnf
ClosedLiteral = "#" ( ListLit | StructLit ) .
```

`#` must immediately precede the bracket, with no space or comment in
between.

A definition may be named `#` today, and `#[0]` indexes it. With the
experiment, `#[` always starts a closed list, so such an index or slice is
written `(#)[0]` or `(#)[0:1]`. `#{` needs no rule, as an identifier followed
by a struct literal is invalid today. Fields named `#` mostly exist to
declare definitions with generated names; a dynamic definition field such
as `#(name): T` could serve that purpose later.

`##[` and `##{` remain invalid, reserved for a possible recursive form.

#### Why one level

On its own terms a recursive `#` reads more naturally: definitions, whose
names start with `#`, close recursively. JavaScript's records and tuples
proposal is sometimes cited for a deep `#[...]` and `#{...}`, but there
the prefix had to repeat on every nested literal, so a deeply immutable
value was written level by level, exactly as a nested structure closed one
level at a time is written here. Two observations decide for one level:

*   **Printing evaluated values.** Because closedness belongs to values, a
    printer needs to state the closedness of one value where that value is
    printed. A one-level literal does exactly that: each value carries its
    own marker, and nothing else in the tree has to change. A recursive-only
    form cannot express "closed here, children as they are".
*   **Meaning-preserving rewrites.** One level is what a data list means
    today. A recursive form would over-close: with
    `#Schema: [{a: int, b: int | *2}]`, a recursively closed `[{a: 1}]`
    rejects the field `b` the schema adds, while `#[{a: 1}] & #Schema` is
    `[{a: 1, b: 2}]`.

#### The experiment

Everything in this proposal is enabled per file by
`@experiment(openlists)`, available from language version v0.18.0. Files
without it are unaffected. Each list follows the file that created it: a
list literal or builtin result from a file without the experiment stays
closed when it meets data from a file with it.

### Migration

`cue fix --exp=openlists` adds the experiment to a file and rewrites it so
that it keeps its meaning:

*   A list literal without a trailing ellipsis outside a definition becomes
    a closed literal: `[1, 2]` becomes `#[1, 2]`.
*   Inside a definition, a list literal is already closed. It becomes a
    closed literal too, so that the file reads the same with or without
    the experiment and the rewrite follows from the syntax of each file
    alone.
*   Slices and calls to builtins that return a list are wrapped in
    `close(...)`.
*   An index or slice on a definition named `#` gains parentheses: `#[0]`
    becomes `(#)[0]`, and `#[0:1]` becomes `(#)[0:1]`.

We measured the rewrite on CUE's own test suite by enabling the experiment
for every file. Without the rewrite, 21 evaluations that failed now
succeeded, 13 of them by merging lists of different lengths, and none that
succeeded failed. With the rewrite applied, all 21 keep their original
outcome and no other outcome changes.

On a corpus of 17 external CUE projects, enabling the experiment
everywhere affected one project: a JSON Schema `oneOf` over tuples, in a
schema the project depends on, matched two alternatives where it had
matched one. The rewrite restores it; the other 16 projects are
unaffected either way.

Two observations matter for migrating real configurations:

*   **Dependencies.** A configuration may depend on modules its author does
    not control. Since the experiment is per file, a dependency keeps its
    closed lists until its own files enable the experiment.
*   **What tests see.** Merging lists of different lengths turns an error
    into a success. Tests that only check that evaluation succeeds will not
    notice; comparing exported output before and after the rewrite does.

The rewrite works per file and does not depend on other packages. A rule
that left a definition's lists alone where nothing in the package could
open them was measured as well, and also restored every project of the
corpus, writing fewer closed literals; it was dropped because a syntactic
analysis may miss a way in which a definition's list is opened. The
rewrite
cannot type the results of calls other than to builtins and leaves such
calls alone; a list-valued result of a user-defined function is data, and
stays open.

Along the way, an evaluator gap was fixed that this rule depended on: a
selection through a regular field holding a definition (`d: #D`, then
`d.s & {b: 2}`) did not keep the definition's closedness, for structs and
lists alike; it does now, independently of the experiment.

### Compatibility

Without the experiment nothing changes. With it:

*   lists outside definitions merge instead of conflicting;
*   `[_] | [_, _]` and similar disjunctions that tell alternatives apart by
    length can become ambiguous for data lists, as the equivalent struct
    disjunctions are today;
*   an out-of-range index on an open data list is incomplete rather than an
    error, as selecting a missing field of an open struct is;
*   `#[` no longer indexes a definition named `#`: `#[0]` must be written
    `(#)[0]`, which means the same with or without the experiment, and
    which `cue fix --exp=openlists` writes.

Output of `cue export` and `cue def` does not change. As a consequence, a
closed list printed by `cue def` reads back open in a file with the
experiment until printers emit closed literals (see Open questions).

### Open questions

*   **Printing.** `cue def` and non-final export keep stating closedness.
    The command knows the language version it targets, so it can choose
    between `close({...})` and `#{...}`, and between `[...]` and `#[...]`,
    per version; until the experiment is permanent, output is unchanged.
*   **`close()`.** With `#{...}` and `#[...]`, the builtin is no longer
    needed and could be deprecated over time, and removed in v1.
*   **A recursive form.** Whether `##[...]` and `##{...}` are needed.
    Reviewers lean against: `#{...}` can be written at every level, and a
    definition closes recursively where that gets cumbersome.
*   **Dynamic definition fields.** Whether `#(name): T` should replace
    fields named `#` (cue-lang/cue#3816). This would also serve the JSON
    Schema and OpenAPI decoders, which emit `#: "foo-bar":` today because
    `#"foo-bar"` is illegal.
*   **Schema conversions.** How `matchN`, the JSON Schema and OpenAPI
    conversions should use open lists and closed literals; a tuple `oneOf`
    is where the corpus showed a difference. Relatedly, lists passed to a
    builtin inside a definition are not closed by the definition.
*   **Ambiguous disjunctions.** Discriminating disjunctions by length is a
    problem lists now share with structs; a remedy would address both.
*   **Inline selection.** Selecting a field from an inline unification with
    a definition loses the definition's closedness for structs and lists
    alike: with `#D: l: {a: int}`, `(#D & {l: {a: 1}}).l & {b: 2}` succeeds
    today, although `#D` closes `l`. This is an evaluator issue independent
    of this proposal.

### Alternatives Considered

#### Keep lists closed and add a schema-level length type

A type such as `list(T, len=3)` or `[3]T` states lengths in schemas, where
lists in definitions are already closed. It does not help data, where the
asymmetry and the round-trip problem live.

#### A recursive `#[...]`, with `close()` for one level

This was the starting point. It reads more naturally, but over-closes
rewritten data and cannot state the closedness of one value (see "Why one
level").

#### `#[...]` as shorthand for `close([...])`

Under the experiment `close` on a list uses the struct mechanism, so
`#[...]` would differ from today's closed list in error wording and in the
cases where closedness is lost through references. Making it exactly
today's list keeps the rewrite meaning-preserving.

#### Rewriting with `close([...])` or a trailing `..._|_`

Wrapping list literals in `close` loses the closedness a definition gives
the list's elements, and breaks lists that refer to their own elements.
Appending `..._|_` works, and is what `#[...]` means, but exposes a
mechanism rather than an intent.

#### Other spellings

Tuples `(a, b)` read well but make `(a,)` error-prone and overload
parentheses. `%[...]` has little precedent. `#*[...]` suggests a default.

### Comparison with other languages

No mainstream language has lists that grow by unification. The nearest
precedents mark a fixed or immutable sequence with a prefix or a separate
literal form:

| Syntax | Language | Meaning | Depth |
|--------|----------|---------|-------|
| `#(1 2 3)` | Scheme, Common Lisp | vector, unlike a list | one level |
| `#[1, 2]`, `#{a: 1}` | JavaScript (records and tuples proposal, withdrawn) | immutable tuple and record | deep, prefix repeated on nested values |
| `%User{...}` | Elixir | struct rejecting unknown keys | one level |
| `(a, b)` | Python, Rust, Swift | tuple | one level |
| `[1, 2] as const` | TypeScript | readonly tuple | deep |
| `List(1, 2)` vs `Listing` | Pkl | closed list vs amendable listing | one level |
