# Proposal: Tagged String Literals

Roger Peppe

Date: 2026-03-05

Discussion at https://github.com/cue-lang/cue/discussions/4295.

## Abstract

We propose adding *tagged string literals* to CUE, a syntax that allows
a function to control how interpolated values are incorporated into a
string. This enables context-aware quoting and escaping, preventing
injection attacks and producing correct output for domain-specific
languages such as shell scripts, Go source code, SQL queries, and HTML.

## Background

CUE's string interpolation (`"hello \(name)"`) concatenates string
representations of values into a string. This works well for simple
cases, but when the resulting string is interpreted by another language,
naive concatenation can produce incorrect or dangerous output.

Consider generating a shell script that includes a filename from a CUE
value:

```cue
filename: "hell 'o.cue \" $foo"
script: "ls -l '\(filename)'"
```

The resulting string is `ls -l 'hell 'o.cue " $foo'`, which is broken
shell syntax. The filename contains a single quote that terminates the
shell string literal prematurely. To produce correct output, the
interpolated value must be escaped according to the shell's quoting
rules, and those rules differ depending on whether the value appears
inside single quotes, double quotes, or unquoted context.

This problem is pervasive. It arises with SQL (SQL injection), HTML
(cross-site scripting), Go source code, YAML, and any other language
embedded in a CUE string. Users must currently escape values manually,
which is error-prone and often done incorrectly.

JavaScript addressed the same problem with [tagged
templates](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Template_literals#tagged_templates),
which allow a function to process the literal and interpolated parts of
a template string separately. We propose a similar mechanism for CUE.

## Proposal

### Syntax

A tagged string literal is written as an expression followed
immediately by a string literal (including multi-line strings):

```cue
tag "string with \(interpolations)"
```

The expression before the string is the *tag*. It must evaluate to a
function. The function receives the string's literal parts and
interpolated values separately, and returns the final string.

Here is the shell example from above, written with a tagged string:

```cue
import "sh"

filename: "hell 'o.cue \" $foo"
script: sh.Format "ls -l 'foo \(filename)'"
```

The `sh.Format` function receives the literal parts (`"ls -l 'foo "`
and `"'"`) and the interpolated values (`filename`), analyzes the shell
syntax to determine the quoting context of each interpolation, and
applies the appropriate escaping. The result is correct shell code
regardless of what `filename` contains.

### Tag function signature

A tag function receives two arguments:

```
tag([...string], [..._])
```

The first argument contains the literal parts of the string, split
at each interpolation point. For a string with N interpolations, there
are N+1 string parts. The second argument contains the N interpolated
values, in order.

For example, given:

```cue
f "hello \(x) world \(y) end"
```

The function `f` is called with:

```
strings: ["hello ", " world ", " end"]
values:  [x, y]
```

The tag function returns a single string value containing the assembled
result.

### Multi-line and alternate-quote strings

Tagged strings work with all CUE string literal forms, including
multi-line strings and alternate-quote strings:

```cue
sh.Format """
    set -e
    cd \(dir)
    make \(target)
    """
```

```cue
go.Format #"""
    fmt.Printf("%s\n", \#(msg))
    """#
```

### Nesting

Tagged strings can be nested, with each layer applying its own
escaping rules. This composes naturally for multi-language scenarios.
Here is a simplified example that generates a shell command containing
Go source code:

```cue
import (
    go "gosource"
    "sh"
)

msg: #"A message containing "quotes" and 'apostrophes'"#

script: sh.Format """
    echo -n \(go.Format #"""
        package main
        import "fmt"
        func main() {
            fmt.Println(\#(msg))
        }
        """#) > main.go
    go run .
    """
```

Each layer of tagging escapes its interpolated values for its own
context. The Go formatter produces valid Go source with properly escaped
string literals. The shell formatter then escapes that entire Go
program for inclusion in a shell string. The final output is a correct
shell script that writes a correct Go program.

### Standard library formatters

We propose initially including formatters for the most common
embedding scenarios:

**`sh.Format`**: Shell-aware string interpolation. Detects whether each
interpolation point is in unquoted, single-quoted, or double-quoted
context and applies the appropriate escaping.

**`go.Format`**: Go-source-aware string interpolation. Detects
whether each interpolation point is in a Go string literal or other
context and applies appropriate escaping or value formatting.

**`html.Format`**: HTML-aware string interpolation. Uses Go's
`html/template` package to apply context-aware escaping, preventing
cross-site scripting.

Additional formatters (SQL, YAML, and others) can be added in future
proposals.

### User-provided tag functions

Since a tag function is an ordinary CUE function, users can provide
their own tag functions using the [user-provided functions
API](../4293-user-functions-and-validators.md) and the [value injection
mechanism](../4294-value-injection.md). This enables domain-specific
formatters without changes to CUE itself:

```go
sqlFormat := cue.PureFunc2(func(parts []string, values []any) (string, error) {
    // Build parameterized SQL query
    // ...
})
j.Register("example.com/sql/format", sqlFormat)
```

```cue
-- sql/sqlformat.cue --
@extern(inject)

package sql

Format: _ @inject(name="example.com/sql/format")

-- db.cue --
package db

import "example.com/sql"

query: sql.Format "SELECT * FROM users WHERE name = \(userName)"
```

## Rationale

### Why not just provide escape functions?

An alternative is to provide functions like `sh.Quote(x)` that users
call explicitly within regular interpolations:

```cue
script: "ls -l '\(sh.Quote(filename))'"
```

This works, but it has several drawbacks. The user must remember to call
the escape function at every interpolation point; forgetting even once
creates a potential injection vulnerability. The escaping is also
context-unaware: a single `sh.Quote` function cannot know whether the
value appears in single quotes, double quotes, or unquoted context, so
it must use the most conservative (and often least readable) escaping.

Tagged strings make correct escaping the default. The tag function sees
the full template structure and can apply context-appropriate escaping
automatically.

### Why this calling convention?

Passing literal parts and interpolated values separately (rather than,
say, a single pre-concatenated string) is what makes context-aware
escaping possible. The tag function can parse the literal parts to
understand the syntactic context of each interpolation point. This is
the same convention used by JavaScript's tagged templates, where it has
proven effective.

### Why not a macro or syntax transformation?

A tagged string is evaluated at runtime, not at parse time. This keeps
the semantics simple: a tagged string is syntactic sugar for a function
call, and the result is an ordinary CUE value. There are no new
evaluation rules or special forms to learn.

## Compatibility

This proposal introduces new syntax. The token sequence
`expr "string"` is currently a syntax error in CUE (an expression
cannot be immediately followed by a string literal), so no existing
valid CUE code is affected.

The evaluation semantics for all existing CUE constructs are unchanged.
Tagged strings desugar to function calls, which follow the existing
rules for function evaluation.

## Implementation

The implementation touches the following areas:

- **Parser** (`cue/parser`): Recognizes the `expr string` pattern in
  primary expression tails and produces a `TaggedInterpolation` AST
  node.
- **AST** (`cue/ast`): Adds the `TaggedInterpolation` type, containing
  a `Tag` expression and an `Str` interpolation.
- **Compiler** (`internal/core/compile`): Compiles `TaggedInterpolation`
  nodes into an internal `adt.TaggedInterpolation` expression that
  separates literal parts from interpolated expressions.
- **Evaluator** (`internal/core/adt`): The `TaggedInterpolation`
  expression evaluates by collecting the literal strings and
  interpolated values into two lists, then calling the tag function
  with those lists.
- **Standard library** (`pkg/sh`, `pkg/gosource`, `pkg/html`): Initial
  set of tag functions for shell, Go source, and HTML.

A proof-of-concept implementation exists, including the three standard
library formatters.

## Open issues

- **Error reporting.** When a tag function returns an error, the error
  currently surfaces as a generic evaluation error. It may be useful to
  include the source location of the tagged string in the error message.

- **Non-string results.** The current design requires tag functions to
  return strings. There may be use cases for returning structured
  values, though it is not clear that tagged string syntax is the right
  mechanism for that.

- **Performance.** For tag functions that parse their literal parts
  (such as the shell formatter), there is a cost per evaluation.
  Caching parsed templates could improve performance but adds
  complexity. This can be addressed later if profiling shows it to be
  a concern.

- **Standard library scope.** The initial set of formatters (shell, Go
  source, HTML) covers common cases but is not exhaustive. SQL
  formatting in particular is a frequently requested feature but
  requires careful design due to the variety of SQL dialects.
