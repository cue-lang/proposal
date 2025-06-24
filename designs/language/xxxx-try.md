# Try Construct and Optional Fields Proposal

*   **Status**: Under Review
*   **Author(s)**: name1@ name2@
*   **Discussion Channel**: {link}

### Objective

This proposal aims to **simplify and reduce verbosity in handling optional fields in CUE** by introducing the `try` comprehension clause and the use of `?` in field references. The `try` construct is intended to provide a more concise syntax for handling optional fields **without the risk of unintentionally swallowing errors** that can occur with current methods. This enhancement also seeks to **improve readability and maintainability** in CUE configurations. The `?` notation further lends itself to **automated discovery of errors** and allows CUE queries to have **the same semantics as usual references**, which leads to **more robust and error-free querying** compared to other query languages.

### Background

In CUE configurations, the conditional definition of fields based on the presence of possibly optional fields is a common pattern. The current approach often necessitates **explicit checks for field existence**, which can result in verbose and cluttered code. For instance, a basic optional field scenario currently looks like this:
```cue
a?: int
if exists(a) {
  b: a + 1
}
```
In this pattern, `a?: int` defines `a` as an optional field, and the `if exists(a)` clause explicitly checks if `a` is defined before `b` is specified. As configurations grow, this **verbosity hampers readability and maintainability**.

A common but problematic pattern in CUE involves using disjunctions to fall back to a default value if a field is not defined:
```cue
a: foo[a.b] | *"default"
```
The intention here might be that `a.b` could optionally not exist in `foo`. However, a significant issue with this method is its **propensity to "gobble errors"**. If `foo` itself or `a.b` is expected to be present but is absent due to a bug, this expression will silently choose the default without notice, leading to a common source of bugs. This is why CUE is **moving away from using comparison with bottom (`_|_`) as it is imprecise and inconsistent**, favoring builtins like `exists`, `isValid`, and `isConcrete`.

While a workaround using an `if` clause exists, it is **more verbose**:
```cue
a: [
  if exists(foo[a.b]) {
    a: foo[a.b]
  },
  "default",
][0]
```
This definition, using the `exists` builtin, allows for precision in determining what kind of errors are allowed. However, **things get messier when more than one lookup is allowed to fail**. For example, if both `a` and `foo` can fail, but `b` cannot, the required expression becomes significantly more complex due to the need to manage evaluation order within `&&` operations, often necessitating nested `if` statements:
```cue
a: [
  if exists(a) && exists(foo) if exists(foo[a.b]) {
    a: foo[a.b]
  },
  "default",
][0]
```
This construction is **very verbose, and its subtlety can be considered unnecessarily complex, especially for newcomers to CUE**. Such patterns have also been a subject of previous discussions, as highlighted in [CUE Issue #165](https://github.com/cue-lang/cue/issues/165).

The `try` construct is intended to **simplify these patterns by allowing a more concise syntax for handling optional fields without the risk of unintentionally swallowing errors**. The problematic example above could be rewritten more cleanly as:
```cue
a: try { foo?[a?.b]? } else { "default" }
```
This new approach also resolves an issue from the query proposal where queries were interpreted slightly differently from normal references, ensuring that **this new interpretation eliminates that difference**.

### Overview

The proposal introduces two key features to address the challenges outlined above:

*   **`?` in Field References**: A new reference type, `field?`, indicates that the referenced field may not exist. When used within a `try` clause, its absence **will not result in an error**. This notation offers **nice symmetry with the existing use of optional fields (`field?: type`)**.
*   **The `try` Comprehension Clause**: This clause attempts to evaluate its body. If any expression within the `try` block fails specifically due to an **undefined optional field (marked with `?`)**, **the entire block is discarded without producing an error**.

This construct simplifies verbose patterns, allowing for a more concise syntax. The `try` clause is designed to be compatible with, and part of, the query language proposal. This meaning and notation allows CUE queries to have **the same semantics as usual references**, which leads to **more robust and error-free querying**.

### Detailed Design

#### Introduction of `?` in Field References

*   **Syntax**: `field?`.
*   **Meaning**: This notation signifies an attempt to reference `field`. If `field` is undefined, the operation should proceed without error, but **only when used within a `try` clause**.
*   **Usage**: `field?` can be used wherever a field reference is valid, but it **must be within a `try` block** to properly handle cases where the field is undefined. We will use the same meaning and notation down the line for a more general query language proposal.


#### The `try` Comprehension Clause

*   **Definition**: A clause that attempts to evaluate its body and discards it without error if any part fails due to undefined optional fields.
*   **Syntax**:
    ```cue
      Comprehension       = Clauses StructLit  [ "else" StructLit ] .

      Clauses             = StartClause { [ "," ] Clause } .
      StartClause         = ForClause | GuardClause .
      Clause              = StartClause | LetClause .
      ForClause           = "for" identifier [ "," identifier ] "in" Expression .
      GuardClause         = "if" Expression .
      TryClause.          = "try" [ identifier "=" Expr ] .
      LetClause           = "let" identifier "=" Expression  .
    ```
    * A `try` clause can appear at the end of a comprehension, interpreting the query in the final body or it may assing an expression to an identifier, which can then be used in the final body of the comprehension.
    * The `else` clause becomes generally available in the comprehension and is invoked if a comprehension yields no results, allowing for a default value to be specified.
    * A comprehension with an `else` clause can be used as a field value without putting it within it in a `StructLit` or `ListLit`, as it will always yield at least one result. In this case, multiple yielded values are simply unified.
*   **Behavior**:
    *   If all expressions within the `try` block succeed, their results are included in the configuration.
    *   If any expression fails specifically due to an **undefined optional field (used with `?`)**, the entire `try` block is ignored and no further results are yielded in the comprehension.
*   **Equivalence to `if` statements**: `try` is functionally equivalent to an `if` statement where every reference marked with `?` is checked for existence before evaluating the expression.
    For example, `try { x: a? + (b? * c) }` is equivalent to:
    ```cue
    if exists(a) if exists(b) {
      x: a + (b * c)
    }
    ```
    Note that the order of the if clauses matters. This illustrates how `try` simplifies complex conditional checks.
*   **Scoping**: A `try` block establishes a distinct scope for handling optional references. The `?` modifier is associated with its innermost `try` block; in nested `try` blocks, each `?` reference is managed by its nearest enclosing `try`.


#### Example Usage

*   **Simple Optional Field Handling**:
    ```cue
    a?: int
    try {
      b: a? + 1
    }
    ```
    *   **When `a` is defined**: `a?` evaluates to `a`, and `b` is assigned `a + 1`.
    *   **When `a` is undefined**: `a?` does not cause an error, and the `try` block is discarded, meaning `b` is not defined.

*   **Nested Optional Fields**:
    ```cue
    user?: {
      name?: string
    }
    try {
      greeting: "Hello, \(user?.name?)!"
    }
    ```
    *   **When `user` and `user.name` are defined**: `user?.name?` evaluates to the user's name, and `greeting` is assigned the personalized message.
    *   **When `user` or `user.name` is undefined**: The `try` block is discarded, and `greeting` is not defined.

*   **Assigning `try` Blocks to Identifiers in Comprehensions**:
    ```cue
    try x = {value: c? + 1} if x.value > 5 {
      a: x
    }
    ```
    *   **Behavior**: If `c` is defined and `condition` is true, `x.value` is assigned `c + 1` and `a: x` is embedded. If `c` is undefined or `condition` is false, `x` is not defined, and nothing is embedded.

*   **Nested `try` clauses**:
    ```cue
    try {
      a: b? + 1
      try {
        c: d? + 2
      }
    }
    ```
    *   **Behavior**: If `b` is defined, `a` is assigned `b + 1`. If `d` is also defined, `c` is assigned `d + 2`. If either `b` or `d` is undefined, the respective inner `try` block is discarded without error. If `b` is defined, but `d` is not, `c` is simply not defined, and field `a` is defined without error.

#### Implementation Considerations

*   **Error Handling**: Within a `try` block, errors specifically from undefined optional fields (marked with `?`) are suppressed, leading to the discard of the entire block.
*   **Compiler Changes**: The CUE compiler will require modifications to recognize the new `try` clause, manage its scoping rules, and implement the defined error suppression behavior.
*   **Backward Compatibility**: The introduction of `try` and `?` syntax will not affect existing CUE configurations unless they explicitly adopt the new features.

### Future Extensions


#### Conditional Fields

We could allow for conditional fields, for instance:

```cue
a: try { expr }

b: if condition { value }
```
where the fields `a` and `b` would not be defined if the `try` or `if` clauses fail.
This is especially useful in the context of an `else` block, where it would avoid duplication. But arguably it also enhances readability by allowing for a more declarative style of defining fields based on conditions.

However, this syntax is ambiguous if there are multiple LHS fields:
```cue
a: b: try { expr }
```
In this case, it is unclear whether field of `a` would also be dropped if `try` fails. Things would be less ambiguous if we used a dot notation shorthand:
```cue
a.b: try { expr }
```
The dot notation has many other benefits, among which unifying the notation for query and definition. We may propose this in the future.

#### Automatic Detection of Brittle CUE

Generally speaking, if a reference refers to an optional field, it should be marked with a `?` and placed within a `try` block. Otherwise, the field should be considered to be required and should probably be marked as such. In other words, the `try` construct introduces a nice symmetry between field declarations and references that can make it easier to check whether CUE expressions are brittle or not.


### Alternatives Considered

#### Not use `?`

We have considered using a query syntax more like JSON Path and JMESPath, where this approach significantly reduces the ability to be detailed about what is allowed to fail. Also, we lose the symmetry with optional field and the ability to do automatic detection of brittle CUE. Finally, this would mean that the simple selectors like `a.b` would be interpreted differently within the context of a query compared to when it is used as a field reference, which is not desirable.


#### Do not introduce `else` in comprehensions

The introduction of `else` might promote the anti-pattern of many nested `if`-`else` clauses. However, we think the introduction of `try` will reduce the need for `if` considerably. Generally, though, CUE could benefit from a `match` construct, similar to the one used in Nickel, as a means to avoid this anti pattern.

### Conclusion

The introduction of the **`try` comprehension clause and `?` in field references offers a concise and expressive solution for handling optional fields in CUE**. This proposal directly addresses the issues of **verbosity and unintentional error swallowing** prevalent in current methods. By providing **symmetry with existing optional field syntax** (`field?: type`), **enhancing error detection** through explicit `try` usage, and enabling **cleaner code** by reducing boilerplate, this design significantly improves readability and maintainability for CUE users. It establishes a robust mechanism for conditional field definitions without compromising crucial error handling or code clarity.