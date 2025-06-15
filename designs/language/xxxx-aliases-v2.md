# Proposal: Postfix Aliases

**Status:** Under Review<br>
**Lifecycle:** Proposed / Under Review<br>
**Author(s):** mpvl@

## Objective / Abstract

We propose an alternative to CUE aliases that addresses various issues, primarily stemming from the current use of the `=` symbol. The proposal aims to simplify the alias system, make it more consistent with other languages, and better integrate aliases with query syntax.

## Background

The current CUE specification defines seven (!) different kinds of aliases spread over five different syntactic positions, making the system complex and hard for users to learn without intimate knowledge of CUE syntax.

A significant issue is the use of `=` for aliases. This is problematic because:
*   It conflicts with the common interpretation of `=` as assignment in many other languages (like Dhall, Nickel, PKL, KCL), where `field: Type = Value` is a common pattern. CUE itself uses `=` for `let` and within attributes in a more traditional sense.
*   Using `=` for aliases prevents using a more natural notation for potential future features like improved default handling.
*   It poses issues when aliases are interspersed within query or comprehension syntax, as the `=` can look like an assignment.
*   Minor issue: in pattern constraints with regular expression comparators, using `=` requires a space to avoid an illegal token, e.g., `[X= =~"^[A-Z]"]`.
* Users should generally prefer value aliases over field aliases, but the current syntax make field aliases more convenient, leading to confusion. As a reminder, value and field aliases differ subtly. In general, one should use field aliases to refer to recursive types or templates, while using value aliases in most other cases.


## Proposal

### Alternative Syntax for Aliases
We propose replacing all current alias forms with a single form using the `~` syntax as a postfix notation.

The primary proposed syntax for a field alias is:

```cue
field~X: value
```
where `X` is an identifier representing the alias. This alias `X` refers to the position/field where it is declared.

To make up for lost functionality from eliminating other alias types, and to allow referring to different aspects of a field alias `X` (defined as `field~X: value`), we propose the following builtins:

*   `keyOf(X)`: Refers to the name of the field (`"field"` in `field~X: value`).
*   `fieldOf(X)`: Refers to the field itself, similar to current CUE's field alias.
*   `valueOf(X)`: Directly links to the value of the field (`value` in `field~X: value`). Referencing an alias `X` directly (e.g., just `X`) outside of a builtin is equivalent to using `valueOf(X)`.

The distinction between `fieldOf(X)` and `valueOf(X)` can be confusing, and users should generally use value aliases or the direct reference `X` as a shorthand for `valueOf(X)`.

Aliases of fixed and dynamic fields are visible within the scope where the field is defined. Aliases on pattern constraints are visible only within the scope of the value of the field.

A potential future phase could introduce a second form:

```cue
field~(K,V): value
```
where `K` is equivalent to `keyOf(X)` and `V` is equivalent to `valueOf(X)` of Form 1.


### Introduction of `self`

We also propose introducing the `self` keyword to refer to the current scope (the value to the right of the innermost colon `:`). `self` can be used to refer to the top of the file (package scope), which was not possible before. The expressions `x: op(self)` and `x~Self: op(Self)` are equivalent.


As for any other predefined identifier in CUE, we would also define `__self` as a non-shadowable variant.


## Detailed Design and Justification

### Comparison between Current and Proposed Syntax

| **alias type**          | **old syntax**                 | **old ref**   | **new syntax**                               | **new ref**             |
| :---------------------- | :----------------------------- | :------------ | :------------------------------------------- | :---------------------- |
| field                   | F=label: value                 | F             | label~X: value                               | fieldOf(X)              |
| dynamic field           | F=(label): value               | F             | (label)~X: value                             | fieldOf(X)              |
| dynamic label           | (K=expr): value                | K             | (label)~X: value<br>(label)~(K,_): value   | keyOf(X)<br>K           |
| pattern constraint field| F=[expr]: value                | F             | [expr]~X: value                              | fieldOf(X)              |
| pattern constraint label| [K=expr]: value                | K             | [expr]~X: value<br>[expr]~(K,_): value       | keyOf(X)<br>K           |
| value                   | label: V=value                 | V             | label~X: value                               | X<br>valueOf(X)         |
| list element            | x: [V=value] (not implemented) | X             | x~X: [ value ]                               | X.0<br>X[0]                     |

The value alias types works for all field types, including dynamic fields and pattern constraints.

### Usage of `self`

Referring to the top of the file (package scope) This was not possible before. Use self to refer to the top of the file.
```
let Top = self

a: {
    b: Top.foo
}
```
Previously preposed using embedded value aliases. We consider this a cleaner alternative.


### Syntax

The proposed change makes the syntax more regular, with an alias occurring only after the label.

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
    Field       = Label ":" { Label ":" } Expression { attribute } .
    Label       = LabelExpr [ "~" identifier  ].
    LabelExpr   = LabelName [ "?" | "!" ] | "[" Expression "]" .
    LabelName   = identifier | simple_string_lit | "(" Expression ")" .
    ```
Note that this reduces the number of positions on which an alias can occur from
three to one.

### Perceived meaning of `=`

The use of `=` for aliases is often perceived as assignment, which differs from its actual meaning in CUE (binding). Moving to `~` eliminates this unconventional use of `=`. The `~` symbol is not commonly used as an operator and has no other meaning in CUE, which helps clarity.

Aliases are identifiers bound to a field declaration and can be referenced within the scope corresponding to the field's value. An alias bound to a `LabelName` is also visible in the enclosing lexical scope.

### Position of Alias

Placing the alias *after* the field name (postfix notation, `field~X`) is considered clearer than a prefix notation (`X~field`) because it reduces the lookahead needed to understand that an identifier is a field name. For example, seeing `a:` immediately tells you `a` is a field name, regardless of whether `~X` follows. This is particularly relevant in dot notation or query extensions.

### Dot Notation Integration

The use of `=` causes issues with a potential future dot notation (`a.b.c: 1`). Field aliases are often used to refer to outer structs within nested structures. Rewriting existing `=` aliases with dot notation can look strange, as `=` typically binds less strongly than `.` in most languages, e.g., `A=a.B=b: { ... }` could be misread as a `A = a.B = b`.

Using `~` instead of `=` makes integrating aliases with dot notation more feasible, as `a~A.b~B: { ... }` reads more clearly, as `~` is typically perceived as binding stronger than `.` (consider Windows file names, for instance).

### Advantages for Query Syntax

Using `~` instead of `=` makes it more feasible to intersperse aliases within query syntax. Although we use it differently, the idea of this came from JSONata which supports [positional variable binding](https://docs.jsonata.org/path-operators#-positional-variable-binding) and [context variable binding](https://docs.jsonata.org/path-operators#-context-variable-binding) in queries.

These positional binding allows for powerful queries, relating values of multiple elements along the path.


This can increase readability of comprehensions as well. Consider a comprehension involving nested structures:

```cue
members: _

y: [ for members.*~M.children.*~C.address~A {
    id:    keyOf(M) + keyOf(C)
    city:  A.city
}]

// same code NOT using aliases in queries
z: [ for mID, m in members for cID, c in m.children {
    id:   mID + cID
    city: c.address.city
}]
```
where `.*` is short for `.[_]` or `.[string]` which ranges over all elements in a container and where the alias following it binds to each element. This alias is then available downstream in the query path.

In example `y`, the query path is clear, and identifiers `X`, `Y`, and `Z` are directly bound to elements within that single path. In example `z`, one must trace variables (`v1`, `v2`) to see that the nested `for` loops continue the path, not create a cross product. This clarity is desirable, as things can get very messy if there are more than two nested loops. This is something we have seen in real-world CUE configurations.

Using `=` in query paths, e.g., `[for x.foo.[X=_].foo.[Y=_].Z=bar { ... }]`, would look like assignment and be confusing. The `~` syntax allows for consistent alias placement after the path element on both RHS and LHS.

Using the phase 2 form, we could write this as:

```cue
// using new aliases in queries
y: [ for members.*~(mID,_).bar.*~(cID,_).baz~A {
    id:    mID + cID
    city:  A.city
}]
```

But this notation can also be used for more complex queries where we filter downstream elements based on earlier result values.

### Regular Expressions in Pattern Constraints

The issue of requiring a space with `=` aliases in pattern constraints (`[X= =~"^[A-Z]"]`) is avoided with the `~` syntax. The syntax `[=~"^[A-Z]"]~X: { ... }` is proposed and should be correctly tokenized even without a space.


### List Element Aliases

List aliases (`[X=value, X+1]`), which are currently in the specification but not implemented, do not work as straightforwardly with the new syntax. The proposal notes that there is no exact equivalent without supporting more syntax. Possible workarounds or alternatives are being considered, such as allowing `let` in lists, allowing `:` for explicit indices (`[0~X: 1, X, X]`), or using struct notation for lists (`{ 0~X: x, 1: X, 2: X }`). We could also elide the position, as in `[~X: x, X, X]`, implying an alias for the first element.

## Transitioning

The transition would involve using `cue fmt` and `cue fix` to automatically rewrite existing `=` aliases to the new `~` syntax. This rewrite is expected to be straightforward and painless for users. The language version field in `modules.cue` would indicate which syntax is supported. This transition could occur before other design decisions about the newly freed `=` or anchors are finalized.


## Alternatives Considered

Various operators besides `~` were considered, including ```%, ^, ::, <-, `, #, $, \, @```. Many were found problematic or less desirable. Using `#` could cause confusion.

Prefix notation (`X~field`) was considered but rejected in favor of postfix notation (`field~X`) due to the clarity benefits discussed earlier.

For `self`, using a "naked" `~` (e.g., `a: ~.b`) was considered as a shorthand but deemed potentially too cryptic.

Alternative notations for referring to aliases, such as requiring a `~` prefix when referring to `X` (e.g., `b: ~X`), were also considered.

<!--

As a reminder.

```
F=foo: V={
    bar: {
        bar: 1
    }
    x: F.bar // field alias: equivalent to foo.bar
    y: V.bar // value alias: equivalent to bar
}
```
These generally result in the same outcomes, but may differ if a value is used as a closure.
In general: if a reference refers to a recursive "template" or "type", use a field alias, otherwise use a value alias.

-->
