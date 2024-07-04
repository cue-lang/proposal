# Objective / Abstract

We propose a solution for directly loading files of any type as part of CUE
evaluation.

# Background

Users frequently need to load JSON, YAML, or other types of files into their CUE
code. As CUE only supports `import` declarations that reference CUE packages,
users currently resort to the CUE tooling layer (`cue cmd`) to load non-CUE
files, which can be overly complex for their needs. The tooling layer was
introduced to handle external influences that make a configuration non-hermetic,
typically files.

However, files that are part of a CUE module can be considered hermetic. We
aim to make it easier to reference these files.

# Overview / Proposal

We propose the `@embed` attribute for embedding.

```
@extern(embed) // Enable processing of embedding.

package foo

// Load a single JSON file
a: _ @embed(file=foo.json)

// Load all files with a name containing a dot (".") in the images directory
// as binary files.
b: _ @embed(glob=images/*.*, type=binary)
b: [string]: bytes

// Unusual file names may be quoted to prevent
// misinterpretation.
c: _ @embed(file="a file.json")
```

Key aspects:

- Embedding must be enabled by a file-level `@extern(embed)` attribute. This
  allows for quick identification of the use of embeddings by tooling.
- Embedded files can be resolved and interpreted at load time, before
  evaluation: it is a syntactic operation.
- The `@embed` attribute can use a file argument for a single file and a glob
  argument for multiple files.
- By default, files are decoded using the encoding implied by the
file name extension. It's an error if the extension is not known.
  This can be overridden using `type=$filetype`, where `$filetypes` can be
  any file type described in `cue help filetypes`.
- For glob, if the extension is not given, the `type` field is required.


# Detailed Design

## Embedding variants

When an embed attribute refers to a file, the file path is interpreted relative
to the directory containing the embed attribute and may not include ‘.’ or ‘..’
or empty path elements. It is not possible to embed a file that is outside the
containing module.

File paths must be `/`-separated, even if CUE is used on Windows or other OS
that does not use `/`-separated paths.

Multiple `@embed` attributes may be associated with the same field, in which
case each of the respective values are unified.

### `@embed(file=$filepath)`

Specifies a single file to be loaded. The encoding of the file is determined by
the file extension unless overridden by type.

It is an error if the file does not exist.

### `@embed(glob=$pattern)`

An embed attribute with the glob argument embeds any matching file as a map from
file path to embedded file. The `$pattern` is matched according to the syntax
used with [`path.Match`](https://pkg.go.dev/cuelang.org/go/pkg/path#Match).

All files must be of the same type, as identified by the extension. In case the
extension is not fully specified (for example `@embed(glob=file.json*)`), the
type needs to be explicitly specified.

We currently do not support `**` to allow selecting files in arbitrary
subdirectories. To allow for this in the future, we do not allow `**` to appear
in the glob.

Files starting with a ‘.’ are not included. We could later add an option to
allow including those.

## File types

File types, when not derived from the file extension, are indicated with the
`type` argument. The values this argument can take follow that of the `cue help
filetypes`. In summary, a type can specify the encoding, interpretation, or
both.

Initially we will not support the CUE filetype. Support for the `cue+data` file
type, or more generally self-contained CUE files, could wbe added at a later
date.

We will also not support [`.jsonl`](https://jsonlines.org/) or multi-doc `.yaml`
file types initially. Instead these files can be embedded as `type=text` and
decoded via `encoding/json` and `encoding/yaml`.

Unlike the command line, `@embed` does not automatically detect the
interpretation based on the contents. For instance, to interpret a JSON file as
OpenAPI, `openapi` needs to be explicitly in the `type` argument.

Just as on the command line, if the extension neither reflects the encoding nor
the interpretation, they can both be specified in type, such as in
`type=openapi+yaml`.

The interpretation of `type` is already internally implemented in the
[`internal/filetypes`](https://pkg.go.dev/cuelang.org/go/internal/filetypes)
package. This could be exposed via a non-internal package.

In the future we could consider an auto-detect option as is available in the
command line.

We will not support schema-guided decoders, such as text protocol buffer values,
as part of the `@embed` mechanism. In these cases, users will have to load the
files as text and use CUE builtin and CUE evaluation to decode the embedded
files.

## Build information

We propose listing files that are selected for embedding in the
[`cue/build.Instance.EmbedFiles`](https://pkg.go.dev/cuelang.org/go/cue/build#Instance)
field.

## Implementation

The embedding proposal can use the `internal/filetypes` and `internal/encoding`
packages to compute the parameters of the decoding. We should investigate if we
can reuse the `runtime.Interpreter` implementation for processing the
attributes, as it is quite similar, though different, to how the `@extern`
attribute is processed.

# Other Considerations

## Only support bytes for now

We wanted to see if we could support a simpler approach that only supports bytes
and force users to convert bytes to the format they want. However, most of the
converter packages assume UTF-8. This is fine to assume for strings within CUE,
but like package [`cue/load`](https://pkg.go.dev/cuelang.org/go/cue/load), it
should not be assumed when loading files.

We could still support only loading bytes if we ensure that all encoder
functionality properly handles BOMs. We may still want to do that regardless
eventually.

## Supporting `**` in `glob` patterns

We currently do not support `**` in a `glob` pattern to match arbitrary
subdirectories. If we do adopt such a feature in the future, we will likely use
the syntax adopted as part of the [LSP
specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#documentFilter),
here `**` is used to match any number of path segments, including none.

## Parent directories

Map keys generated for the glob option are files relative to the directory
containing the `@embed` attribute. We could, instead, create path keys relative
to a module root. This would make it possible to embed files from parent
directories (as long as they are within the same module). We could make this an
option later on and denote such paths starting them with `/` to represent the
module root.

## Security

Embedding is always enabled and may pull in files that end up being exposed in a
configuration.

The restrictions that disallow embedding files from parent directories, and that
limit any embedding to files within the containing CUE module, preclude the
loading of sensitive files from random places on disk.

A CUE module's
[`source.kind`](https://cuelang.org/docs/reference/modules/#determining-zip-file-contents)
ensures that the contents of a published module correspond to a VCS commit.
Assuming that sensitive files are not included as part of a VCS commit, this
ensures that a published CUE module will also not contain sensitive files.

It is ultimately, however, the responsibility of the module author to ensure
that sensitive files are not accidentally included.
