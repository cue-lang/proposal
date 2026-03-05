# Proposal: CUE Value Injection

Roger Peppe

Date: 2026-03-05

Discussion at https://github.com/cue-lang/cue/discussions/4294.

## Abstract

We propose an injection mechanism that allows Go code to supply CUE
values to specific locations in CUE source, identified by `@inject`
attributes. This provides a controlled, explicit way for host programs
to parameterize CUE configurations with values that cannot be expressed
in CUE alone, such as user-provided functions and validators.

## Background

Go programs that use CUE often need to supply values that originate
outside the CUE configuration. Today this is done primarily through
`Value.FillPath`, which works well for simple cases but has
limitations. The calling code must know the exact path where a value
should be placed, which creates tight coupling between Go code and CUE
structure. When a CUE package wants to declare that it expects an
externally-provided value, there is no standard way to express this
intent.

The companion proposal for [user-provided functions and
validators](./4293-user-functions-and-validators.md) makes injection
particularly important. Functions and validators cannot be expressed in
CUE syntax -- they must come from Go. A CUE package that uses a
custom function needs a way to declare the dependency and have it
fulfilled by the host program.

CUE already has the `@extern` attribute mechanism for integrating with
external code. The injection mechanism builds on this existing
infrastructure, using `@extern(inject)` to opt in at the file level
and `@inject` attributes to mark individual fields.

## Proposal

### The Injector type

We add a new type to the `cue/cuecontext` package:

```go
type Injector struct { ... }

func NewInjector() *Injector
```

An `Injector` maintains a registry of named CUE values and an
authorization function that controls which injections are permitted.

### Registering values

```go
func (j *Injector) Register(name string, v cue.Value)
```

`Register` associates a CUE value with a name. If a value is already
registered under that name, the new value is unified with the existing
one. This allows multiple registrations to accumulate constraints
incrementally:

```go
j := cuecontext.NewInjector()
j.Register("example.com/config", ctx.CompileString(`{port: int}`))
j.Register("example.com/config", ctx.CompileString(`{port: 8080}`))
// The registered value is now {port: 8080}
```

### Authorization

Injection is a potentially security-sensitive operation: it allows Go
code to override values in CUE configurations. The `Injector` requires
explicit authorization before any injection can take effect.

```go
func (j *Injector) Allow(f func(inst *build.Instance, name string) error)
func (j *Injector) AllowAll()
```

`Allow` sets a function that is called for each injection site. It
receives the build instance (identifying the CUE package) and the
injection name. Returning a non-nil error prevents the injection.
`AllowAll` is a convenience that permits all injections.

If no authorization function is set, all injections fail with an
error. This fail-closed default ensures that injections cannot happen
accidentally.

### Connecting to a Context

```go
func Inject(j *Injector) Option
```

`Inject` returns a `cuecontext.Option` that registers the injector
with a CUE context. The injector is activated as an interpreter for
`@extern(inject)` attributes.

```go
j := cuecontext.NewInjector()
j.AllowAll()
ctx := cuecontext.New(cuecontext.Inject(j))
```

### CUE-side syntax

On the CUE side, a file opts into injection with a file-level
`@extern(inject)` attribute. Individual fields are marked with
`@inject(name="...")`:

```cue
@extern(inject)

package myapp

// validate is a user-provided validator, injected from Go.
validate: _ @inject(name="example.com/myapp/validate")

// config has a default that can be overridden by injection.
config: "default" @inject(name="example.com/myapp/config")
```

When the injector has a value registered for the given name, that value
is unified with the field's CUE-side value. When no value is registered,
the field retains its original value (in the example above, `config`
would remain `"default"` and `validate` would remain `_`).

Without the file-level `@extern(inject)` attribute, `@inject`
attributes are silently ignored. This ensures that injection is an
explicit opt-in at the file level.

### Combining with user-provided functions

The primary motivating use case is injecting functions and validators
from the companion [user-provided functions
proposal](./4293-user-functions-and-validators.md):

```go
j := cuecontext.NewInjector()
j.AllowAll()
ctx := cuecontext.New(cuecontext.Inject(j))

j.Register("example.com/myapp/validate",
    cue.ValidatorFunc(func(s string) error {
        if !isValidHostname(s) {
            return fmt.Errorf("invalid hostname: %q", s)
        }
        return nil
    }))
```

```cue
@extern(inject)

package myapp

#validHost: _ @inject(name="example.com/myapp/validate")

servers: [...{
    host: #validHost & string
}]
```

This pattern cleanly separates concerns: the CUE code declares that it
expects an external validator and specifies where it should be applied,
while the Go code provides the implementation.

## Rationale

### Why not just use FillPath?

`FillPath` requires the Go code to know the exact CUE path where a
value should be placed. This works for simple top-level values but
becomes fragile as CUE structure evolves. The injection mechanism
inverts the dependency: the CUE code declares what it needs, and the Go
code provides it by name.

More importantly, `FillPath` operates on already-compiled CUE values.
Injection happens during compilation, which means injected values
participate fully in CUE's evaluation from the start. This is
particularly important for functions, which need to be available when
expressions referencing them are first evaluated.

### Why require @extern(inject)?

The file-level opt-in serves two purposes. First, it makes injection
visible at a glance: a reader can see immediately that a file depends
on external values. Second, it integrates with CUE's existing
`@extern` infrastructure, which is designed precisely for this kind of
external integration.

### Why an authorization function?

Injection allows Go code to substitute values into CUE configurations.
In environments where CUE configurations come from untrusted sources
(for example, user-submitted schemas), unrestricted injection could be
a vector for unexpected behavior. The authorization function gives the
host program fine-grained control over what can be injected and where.

The fail-closed default (requiring explicit `Allow` or `AllowAll`)
ensures that a forgotten authorization step results in clear errors
rather than silent security holes.

### Why use names rather than paths?

Names decouple the injection site from its position in the CUE value
tree. A name like `"example.com/myapp/validate"` is stable even if the
CUE code is refactored. It also allows the same value to be injected
at multiple sites by using the same name.

## Compatibility

This proposal adds new public API to the `cue/cuecontext` package. It
does not change existing CUE evaluation semantics. The `@inject`
attribute is new; existing code that does not use `@extern(inject)` is
entirely unaffected.

The `@extern(inject)` attribute uses the existing `@extern`
infrastructure, which is designed to be extensible with new interpreter
kinds.

## Implementation

The implementation adds:

- `cue/cuecontext/inject.go`: The `Injector` type and its methods,
  plus the `injectInterpreter` and `injectCompiler` types that
  integrate with `runtime.Interpreter`.
- The injector compiles `@inject` attributes by looking up the
  registered value and returning it as an `adt.Expr`, or returning
  `&adt.Top{}` when no value is registered.

A proof-of-concept implementation exists.

## Open issues

- **Name structure.** The current design uses opaque strings as
  injection names. It may be worth imposing structure on these names,
  for example requiring them to follow a URL-like or module-path-like
  convention. This would make authorization policies easier to express
  (allowing all injections from a particular domain) and reduce the
  risk of name collisions.

- **Versioning.** If injection names follow a structured convention,
  it may be useful to include version information, allowing a CUE
  package to declare that it requires a particular version of an
  injected interface.

- **Discoverability.** There is currently no way for a Go program to
  discover what injections a CUE package expects without inspecting
  the source. A future mechanism for declaring injection requirements
  in module metadata could address this.
