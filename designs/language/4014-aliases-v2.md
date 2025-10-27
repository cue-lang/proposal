# Proposal: Postfix Aliases

**Status:** Under Review<br>
**Lifecycle:** Proposed / Under Review<br>
**Author(s):** mpvl@

## Objective / Abstract

We propose an alternative to CUE aliases that addresses various issues,
primarily stemming from the current use of the `=` symbol.
The proposal aims to simplify the alias system, make it more consistent with
other languages, and better integrate aliases with query syntax.

## Background

The current CUE specification defines [seven (!) different kinds of aliases](https://cuelang.org/docs/references/spec/#aliases)
spread over five different syntactic positions, making the system complex and
hard for users to learn without intimate knowledge of CUE syntax.

A significant issue is the use of `=` for aliases.
This is problematic because:
* It conflicts with the common interpretation of `=` as assignment in many other
  languages (like Dhall, Nickel, PKL, KCL), where `field: Type = Value` is a
  common pattern.
  CUE itself uses `=`for `let` and within attributes in a more traditional sense.
* Using `=` for aliases prevents using a more natural notation for potential
  future features likeimproved default handling.
* It poses issues when aliases are interspersed within query or comprehension
  syntax, as the `=`can look like an assignment.
* Minor issue: in pattern constraints with regular expression comparators, using
  `=` requires aspace to avoid an illegal token, e.g., `[X= =~"^[A-Z]"]`.
* Users should generally need the value of a field rather than a reference to
  the field itself, but the current syntax makes field aliases more convenient,
  leading to confusion. As a reminder, value and field aliases differ subtly.
  In general, one should use field aliases to refer to recursive types,
  while using value aliases in most other cases.

Additionally, the semantics of the aliases is hard to get right.
Evidence for this can be found in various open issues, such as:
* [Issue 2194](https://github.com/cue-lang/cue/issues/2194)
* [Issue 2506](https://github.com/cue-lang/cue/issues/2506)
* [Issue 2736](https://github.com/cue-lang/cue/issues/2736)

## Proposal

### Alternative Syntax for Aliases
We propose replacing all current alias forms with a single form using the `~`
syntax as a postfix notation.
Other aspects of aliases, such as scoping rules, will remain the same.

The primary proposed syntax for a field alias is:

```cue
field~X: value
```
where `X` is an identifier representing the alias. This alias `X` refers to the
position/field where it is declared.
In this example, both the field name and the alias `X` are brought into scope.

With the new alias syntax, a field alias `X` (defined as `field~X: value`)
always refers to the field itself, similar to current CUE's field alias.
This means a reference to the field declaration position, which when evaluated
includes both the field's structure and any constraints defined on it.
This is useful for recursive types.

To obtain the value of a field (what was previously called a "value alias"),
use `let` with `self`:

```cue
field~X: {
    let V = self  // V now holds the value of the field
    // X refers to the field itself
    // V refers to the value
}
```

We also propose a second form for field aliases that captures both the key and
the field reference:

```cue
field~(K,V): value
```
where `K` refers to the name of the field (`"field"` in this example) and `V`
refers to the field itself (equivalent to the single-identifier form `field~V`).

For example:
```cue
x~(K,V): {
    y: K  // evaluates to "x"
    z: V  // refers to field x itself
}
```

Aliases of fixed and dynamic fields are visible within the scope in which the
field is defined.
Aliases on pattern constraints are visible only within the scope of the value of
the field.


### Introduction of `self`

We also propose introducing the `self` keyword to refer to the current
_block_, which is:
- the top of the file (package scope)
- the innermost `{}` block
- this includes elided blocks, so `a: b: self.foo`
is equivalent to `a: {b: self.foo}`.

Note that `self` refers to the innermost enclosing block.
For example, in `let x = self`,
`self` refers to the scope in which `let` is defined.
To bind with

With field aliases always referring to the field, `let V = self` becomes the
alternative of choice for value aliases.

As for any other predefined identifier in CUE, we would also define `__self` as
a non-shadowable variant.


## Detailed Design and Justification

### Comparison between Current and Proposed Syntax

Here are side-by-side examples showing how real-world CUE code would change:

#### Field Alias Example
```cue
// Old syntax
data=Config: {
    host: "localhost"
    port: 8080
    url: "http://\(data.host):\(data.port)"
}

// New syntax
Config~data: {
    host: *"localhost" | string
    port: 8080
    url: "http://\(data.host):\(data.port)"  // data refers to the field itself
}

// Example showing data.host refers to original value
config: Config
config: host: "example.com"  // host is unified to "example.com"
// data.host still refers to the original constraint (*"localhost" | string), not "example.com"
```

#### Value Alias Example
```cue
settings: timeout: 30
settings: #Settings

#Settings: timeout: int

// Old syntax
#Settings: X={
    retry: X.timeout * 2
}
// New syntax
#Settings: {
    let X = self
    retry: X.timeout * 2

    // OR

    retry: self.timeout * 2
}
```
Note that using a field alias on `#Settings` would not give the desired
result in this case, as it would refer to `#Settings.timeout`, not the
given value in `settings`.

#### Validator Example
When we introduce `must`, or other validators, it will be necessary to refer to
the current value. Since `self` binds to the inner block, this means that we
need to put the validators themselves in a block:
```cue
import "strconv"

// a is a string respresenting an integer
a: {must(strconv.Atoi(self))}
```
Note that the curly braces around the validator are necessary to resolve `self`
correctly to `a`.

#### Comprehension Example
Note that a `self` in an "embedded" struct will end up referring to the value
in which it is embedded.
```cue

a: {
    if true {
        x: self.y // resolves to 3
    }
}
a: y: 3
```
Here, `self` is scoped by the block associated with `if`.
This block will ultimately unify with `a`, meaning that `self.y` resolves to `3`.

#### Pattern Constraint Example
```cue
// Old syntax
[K=string]: {
    name: K
    type: "dynamic"
}

// New syntax
[string]~(K,_): {
    name: K
    type: "dynamic"
}
```

#### Summary of equivalent forms

| **alias type**          | **old syntax**                 | **old ref**   | **new syntax**                               | **new ref**             |
| :---------------------- | :----------------------------- | :------------ | :------------------------------------------- | :---------------------- |
| field                   | `F=label: value`                 | `F`             | `label~X: value`                               | `X`                     |
| dynamic field           | `F=(label): value`               | `F`             | `(label)~X: value`                             | `X`                     |
| dynamic label           | `(K=expr): value`                | `K`             | `(label)~(K,V): value`                       | `K`                     |
| pattern constraint field| `F=[expr]: value`                | `F`             | `[expr]~X: value`                              | `X`                     |
| pattern constraint label| `[K=expr]: value`                | `K`             | `[expr]~(K,V): value`                        | `K`                     |
| value                   | `label: V=value`                 | `V`             | `label: {let V = self, value }`              | `V`                     |
| list element            | `x: [V=value, V+1]` (not implemented) | `V`             | (no direct equivalent - see note below)       | N/A                        |

Value aliases can be created for all field types using `let V = self`, including
for dynamic fields and pattern constraints.

Note: for the `(K,V)` variant, if only the key is needed, `V` can be
replaced with `_` as a blank identifier to indicate the value is unused.


### Usage of `self`

Referring to the root of the file (package scope) was not possible before.
Use self to refer to the root of the file.
```
let Root = self

a: {
    b: Root.foo
}
```
We previously proposed using embedded value aliases.
We consider this a cleaner alternative.


### Syntax

The proposed change makes the syntax more regular, with an alias occurring only
after the label.

*   **Current Syntax (Excerpts):**
    ```ebnf
    AliasExpr   = [ identifier "=" ] Expression .
    Field       = Label ":" { Label ":" } AliasExpr { attribute } .
    Label       = [ identifier "=" ] LabelExpr .
    LabelExpr   = LabelName [ "?" | "!" ] | "[" AliasExpr "]" .
    LabelName   = identifier | simple_string_lit | "(" AliasExpr ")" .
    ```


*   **Proposed Syntax (Excerpts):**
    ```ebnf
    Field        = Label ":" { Label ":" } Expression { attribute } .
    Label        = AliasedLabel [ "?" | "!" ] | AliasedConstraint .
    AliasedLabel = LabelName [ "~" identifier ] .
    AliasedConstraint = "[" Expression "]" [ "~" identifier ] .
    LabelName    = identifier | simple_string_lit | "(" Expression ")" .
    ```

For optional and required fields, the alias comes after the field name but
before the constraint marker. For pattern constraints, the alias comes after 
the closing bracket. For example:
```cue
// Optional field with alias
optional~X?: foo

// Required field with alias
required~Y!: bar

// Pattern constraint with alias (both forms supported)
[string]~X: value          // X refers to the field
[string]~(K,V): value      // K is the key, V refers to the field
```

This syntax was chosen over `optional?~X: foo` to make it clearer that `~`
binds to the field name, not to a composite operator like `?~`.

Note that this reduces the number of positions on which an alias can occur from
three to one.

### Perceived meaning of `=`

The use of `=` for aliases is often perceived as assignment, which differs from
its actual meaning in CUE (binding).
Moving to `~` eliminates this unconventional use of `=`.
The `~` symbol is not commonly used as an operator, which helps clarity.
It is also used for regular expressions, but this use case seems different
enough to not cause confusion.

Aliases are identifiers bound to a field declaration and can be referenced
within the scope corresponding to the field's value.
An alias bound to a `LabelName` is also visible in the enclosing lexical scope.

### Position of Alias

Placing the alias *after* the field name (postfix notation, `field~X`) is
considered clearer than a prefix notation (`X~field`) because it reduces the
lookahead needed to understand that an identifier is a field name.
For example, regardless of whether one writes `a: x` or `a~X: x ` one can see
form the start of the field that `a` is the name.
This is particularly relevant in dot notation or query extensions.

### Dot Notation Integration

The use of `=` causes issues with a potential future dot notation (`a.b.c: 1`).
Field aliases are often used to refer to outer structs within nested structures.
Rewriting existing `=` aliases with dot notation can look strange, as `=`
typically binds less strongly than `.` in most languages, e.g., `A=a.B=b: { ...
}` could be misread as a `A = a.B = b`.

Using `~` instead of `=` makes integrating aliases with dot notation more
feasible, as `a~A.b~B: { ... }` reads more clearly, as `~` is typically
perceived as binding stronger than `.` (consider Windows file names, for
instance).

### Advantages for Query Syntax

Using `~` instead of `=` makes it more feasible to intersperse aliases within
query syntax. Although we use it differently, the idea of this came from JSONata
which supports [positional variable
binding](https://docs.jsonata.org/path-operators#-positional-variable-binding)
and [context variable
binding](https://docs.jsonata.org/path-operators#-context-variable-binding) in
queries.

These positional binding allows for powerful queries, relating values of
multiple elements along the path.


This can increase readability of comprehensions as well.
Consider a comprehension involving nested structures:

```cue
members: _

y: [ for members.*~(mID,M).children.*~(cID,C).address~A {
    id:    mID + cID
    city:  A.city
}]

// same code NOT using aliases in queries
z: [ for mID, m in members for cID, c in m.children {
    id:   mID + cID
    city: c.address.city
}]
```
where `.*` is short for `.[_]` or `.[string]` which ranges over all elements in
a container and where the alias following it binds to each element.
This alias is then available downstream in the query path.

In example `y`, the query path is clear, and identifiers `X`, `Y`, and `Z` are
directly bound to elements within that single path.
In example `z`, one must trace variables (`v1`, `v2`) to see that the nested
`for` loops continue the path, not create a cross product.
This clarity is desirable, as things can get very messy if there are more than
two nested loops.
This is something we have seen in real-world CUE configurations.

Using `=` in query paths, e.g., `[for x.foo.[X=_].foo.[Y=_].Z=bar { ... }]`,
would look like assignment and be confusing.
The `~` syntax allows for consistent alias placement after the path element on
both RHS and LHS.

Using the phase 2 form, we could write this as:

```cue
// using new aliases in queries
y: [ for members.*~(mID,_).bar.*~(cID,_).baz~A {
    id:    mID + cID
    city:  A.city
}]
```

But this notation can also be used for more complex queries where we filter
downstream elements based on earlier result values.

### Regular Expressions in Pattern Constraints

The issue of requiring a space with `=` aliases in pattern constraints (`[X=
=~"^[A-Z]"]`) is avoided with the `~` syntax. The syntax `[=~"^[A-Z]"]~X: { ...
}` is proposed and should be correctly tokenized even without a space.


### List Element Aliases

List element aliases (e.g., `x: [V=value, V+1]` in the old syntax), which are
currently in the CUE specification but not implemented, do not have a direct
equivalent in the new syntax. The old syntax allowed binding an alias to a
list element that could be referenced within the list literal itself.

Possible workarounds or alternatives being considered include:
- Allowing `let` expressions within lists
- Supporting explicit index notation with aliases: `[0~X: 1, X, X]`
- Using struct notation for lists: `{ 0~X: x, 1: X, 2: X }`
- Eliding the position for the first element: `[~X: x, X, X]`

Until a solution is chosen, this specific alias type will not be supported in
the new syntax.

## Transitioning

The transition would involve using `cue fmt` and `cue fix` to automatically
rewrite existing `=` aliases to the new `~` syntax. This rewrite is expected to
be straightforward and painless for users.
The language version field in `modules.cue` would indicate which syntax is
supported.
The intent is to support both the old and new syntax for a transition
period&mdash;for instance two minor versions&mdash;allowing users to gradually
update their codebases.
This transition could occur before other design decisions about the newly freed
`=` or anchors are finalized.


## Alternatives Considered

Various operators besides `~` were considered, including ```%, ^, ::, <-, `, #,
$, \, @```.
Many were found problematic or less desirable. Using `#` could cause confusion.

Prefix notation (`X~field`) was considered but rejected in favor of postfix
notation (`field~X`) due to the clarity benefits discussed earlier.

For `self`, using a "naked" `~` (e.g., `a: ~.b`) was considered as a shorthand
but deemed potentially too cryptic.

Alternative notations for referring to aliases, such as requiring a `~` prefix
when referring to `X` (e.g., `b: ~X`), were also considered.

<!--

As a reminder.

```
// Old syntax
F=foo: V={
    bar: {
        bar: 1
    }
    x: F.bar // field alias: equivalent to foo.bar
    y: V.bar // value alias: equivalent to bar
}

// New syntax
foo~F: {
    let V = self
    bar: {
        bar: 1
    }
    x: F.bar // field alias: equivalent to foo.bar
    y: V.bar // value alias: equivalent to bar
}
```
These generally result in the same outcomes, but may differ if a value is used as a closure.
In general: if a reference refers to a recursive "type", use a field alias, otherwise use `let V = self` for a value alias.

-->
