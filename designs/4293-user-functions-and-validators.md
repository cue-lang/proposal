# Proposal: User-Provided Functions and Validators

Roger Peppe

Date: 2026-03-05

Discussion at https://github.com/cue-lang/cue/discussions/4293.

## Abstract

We propose adding a Go API for creating CUE-callable functions and
validators from ordinary Go functions. This provides a simple, type-safe
mechanism for extending CUE's evaluation with custom logic, without
requiring the complexity of the existing built-in function infrastructure.

## Background

CUE's built-in functions (in the `pkg/` directory) are implemented using
an internal framework that is tightly coupled to the evaluator. Adding
new built-in functions requires modifying internal packages, running code
generators, and understanding the details of the `adt` package. This
makes it impractical for users who want to extend CUE with custom logic
from their Go programs.

There is significant demand for user-defined functions. Common use cases
include custom validation logic (checking values against external
databases, applying domain-specific rules), data transformation
(encoding, hashing, formatting), and integration with external services.
Currently, the only way to achieve this is to evaluate CUE, extract
values in Go, process them, and feed results back -- a workflow that is
both verbose and loses the benefits of CUE's constraint-based evaluation.

The existing built-in function system also has some design choices that
we would prefer not to carry forward into a user-facing API. In
particular, some built-ins conflate functions and validators: for
example, `list.UniqueItems` can be used both as a bare constraint
(`list.UniqueItems`) and as a function call (`list.UniqueItems()`), with
identical semantics. This ambiguity complicates the mental model. Some
built-ins also support optional trailing arguments with default values,
which introduces a form of variable-arity calling that adds complexity.

We propose a simpler model that avoids both of these issues while
remaining fully general.

## Proposal

### Functions

We add a family of generic functions to the `cue` package, one for each
supported argument count:

```go
func NewPureFunc1[A0, R any](f func(A0) (R, error), opts ...FuncOption) Value
func NewPureFunc2[A0, A1, R any](f func(A0, A1) (R, error), opts ...FuncOption) Value
// ... through PureFunc10
```

Each `NewPureFuncN` wraps a Go function of N arguments into a CUE `Value`
that can be called from CUE expressions. The "Pure" prefix indicates
that these functions are *referentially transparent*: calling them with
the same arguments always produces the same result. This property is
essential for CUE's evaluation model, which may evaluate expressions
multiple times or in any order.

Arguments are decoded from CUE values to Go types using `Value.Decode`,
and results are converted back to CUE values. This means that any Go
type supported by `Decode` (including structs, slices, maps, and types
implementing `json.Unmarshaler`) can be used as an argument type, and
any Go type convertible to a CUE value can be used as a return type.

Here is an example of creating and using a simple function:

```go
ctx := cuecontext.New()
v := ctx.CompileString(`#add: _, x: #add(3, 4)`)
v = v.FillPath(cue.ParsePath("#add"), cue.NewPureFunc2(func(a, b int) (int, error) {
    return a + b, nil
}))
fmt.Println(v.LookupPath(cue.ParsePath("x"))) // 7
```

Note that the function is injected into the CUE value via `FillPath`
into a definition (`#add`). This is one way of using them, although
we anticipate that a more common pattern will be to use value
injection (see https://github.com/cue-lang/cue/discussions/4294).

If the Go function returns a non-nil error, the CUE expression
evaluates to bottom with that error message:

```go
v = v.FillPath(cue.ParsePath("#f"), cue.NewPureFunc1(func(x int) (int, error) {
    if x < 0 {
        return 0, fmt.Errorf("negative value not allowed")
    }
    return x * 2, nil
}))
```

If the wrong number of arguments is passed in CUE, evaluation produces
an error.

### Validators

We add a generic function for creating validators:

```go
func NewPureValidatorFunc[T any](f func(T) error, opts ...FuncOption) Value
```

A validator is a CUE value that constrains what it is unified with. When
unified with a concrete value, the validator decodes that value as type
`T` and calls `f` to validate it. If `f` returns a non-nil error, the
unification fails with that error. If `f` returns nil, the value passes
through unchanged.

```go
ctx := cuecontext.New()
v := ctx.CompileString(`#v: _, x: #v & "hello"`)
v = v.FillPath(cue.ParsePath("#v"), cue.ValidatorFunc(func(s string) error {
    if len(s) < 3 {
        return fmt.Errorf("string too short")
    }
    return nil
}))
fmt.Println(v.LookupPath(cue.ParsePath("x"))) // "hello"
```

Validators are distinct from functions. A function computes a new value
from its arguments; a validator constrains an existing value. This
distinction is clear in both the API and the CUE usage: functions are
called with parentheses (`#f(x)`), while validators are unified with
`&` (`#v & x`).

This separation does not reduce generality. A function can return a
validator value, allowing patterns like `#minLen(3) & "hello"` where
`#minLen` is a function that returns a validator. This composes
naturally with CUE's existing constraint model.

### Options

Both `NewPureFuncN` and `ValidatorFunc` accept optional `FuncOption` values.
Currently one option is defined:

```go
func Name(name string) FuncOption
```

`Name` sets the name used in error messages. Without it, error messages
refer to the function anonymously.

### Exporting

User-provided functions and validators cannot currently be exported to
CUE syntax. Attempting to format a value containing a function or
validator produces an error message indicating this limitation. This is
an area for future work.

### Restrictions

This initial proposal deliberately omits several capabilities:

- **Variable-arity functions.** Each `NewPureFuncN` requires exactly N
  arguments. There is no support for optional arguments or variadic
  calls.

- **Non-concrete arguments.** All arguments must be fully
  resolved CUE values. Non-concrete argument values are not currently
  supported, although they could be.

- **Side effects.** Only pure functions are supported. Functions that
  perform I/O or maintain state are not appropriate for use with this
  API, as CUE's evaluator makes no guarantees about evaluation order or
  frequency.

These restrictions keep the initial implementation simple and the
semantics clear. Each could be relaxed in future proposals if there is
demand.

## Rationale

### Why separate functions and validators?

The existing built-in system's conflation of functions and validators
(`list.UniqueItems` vs `list.UniqueItems()`) is a source of confusion.
Separating them in the user-facing API makes the mental model clearer:
functions compute, validators constrain. Since a function can return a
validator, there is no loss of expressiveness.

### Why fixed arity?

Supporting variable-arity functions would require either a
`...interface{}` argument (losing type safety) or a more complex
registration mechanism. Fixed arity with generic type parameters gives
us compile-time type safety in Go while keeping the API surface small.
The maximum of 10 arguments covers the vast majority of practical use
cases.

### Why NewPureFunc and not just NewFunc?

The "Pure" prefix is intentional. CUE's evaluation model assumes that
expressions are deterministic. Making purity explicit in the name serves
as documentation and a reminder. It also leaves room for a hypothetical
future `Func` (or similar) that might support side effects in a
controlled manner, perhaps in the context of CUE scripting.

### Why not use the existing built-in framework?

The existing framework is designed for functions that ship with CUE
itself. It uses code generation, internal types, and conventions that
are not suitable for an external API. A separate, simpler API is more
appropriate for user-provided functions.

## Compatibility

This proposal adds new public API surface to the `cue` package. It does
not change existing CUE evaluation semantics or syntax. No existing code
is affected.

The `NewPureFuncN` naming pattern reserves a range of names in the `cue`
package. We do not expect this to conflict with any likely future
additions.

## Implementation

The implementation adds the following to `cuelang.org/go`:

- `cue/func.go`: The `ValidatorFunc` function and the internal
  `pureFunc` generic implementation.
- `cue/func_gen.go`: Generated `NewPureFunc1` through `NewPureFunc10`
  functions, produced by `cue/generate_func.go`.
- `internal/core/adt`: Two new types, `Func` (implementing the `Expr`
  interface for callable functions) and `FuncValidator` (implementing
  the `Validator` interface). These integrate with the evaluator's
  existing function-call and validation machinery.

A proof-of-concept implementation exists. See
https://cue.gerrithub.io/c/cue-lang/cue/+/1232921. See also the
companion proposals on [value injection](./4294-value-injection.md)
(which provides a mechanism for injecting functions into CUE packages
without `FillPath`) and [tagged string
literals](./language/4295-tagged-string-literals.md) (which
demonstrates a use case for user-provided functions).

## Future work

Several directions could build on this foundation:

- **Function signature declarations.** A CUE syntax for declaring the
  expected signature of a function value, enabling static checking of
  call sites.
- **Keyword arguments.** Support for named arguments, which would
  compose naturally with CUE's struct-based data model.
- **Custom CUE binaries.** A mechanism for building a `cue` binary that
  includes custom function packages, similar to how Caddy allows custom
  modules.
- **RPC protocol.** An IPC-based protocol for executing custom functions
  in a separate process, removing the requirement that functions be
  implemented in Go and enabling language-agnostic extensibility.
