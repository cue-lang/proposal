# Proposal: Support `io/fs.FS` in `cue/load`

Roger Peppe

Date: 2026-02-18

Discussion at [https://github.com/cue-lang/cue/discussions/4285](https://github.com/cue-lang/cue/discussions/4285).

## Abstract

We propose adding an `FS` field of type `io/fs.FS` to
`cue/load.Config`, allowing the CUE loader to read packages and
modules from virtual filesystems rather than the host operating
system. This enables use cases such as embedding CUE modules in Go
binaries via `embed.FS`, running CUE evaluation in sandboxed or server
environments, and testing loaders against in-memory filesystems.

## Background

The CUE loader (`cue/load`) currently reads all source files from the
host filesystem. An `Overlay` mechanism exists that maps absolute file
paths to in-memory contents, layered on top of the real filesystem,
but this has significant limitations. Since overlay paths must be
absolute and are resolved relative to directories that must exist (or
be themselves overlaid), users who want a fully virtual filesystem
must construct plausible absolute paths and ensure directory entries
are consistent. This is fragile and unintuitive, as the [discussion on
\#607](https://github.com/cue-lang/cue/issues/607) demonstrates. See
also [this thread](https://github.com/cue-lang/cue/discussions/1145#discussioncomment-1082053)
which discusses how to load CUE files that have been embedded in a Go
binary.

Go 1.16 introduced the `io/fs.FS` interface, which has since become
the standard abstraction for read-only filesystems in the Go
ecosystem. Types such as `embed.FS`, `fstest.MapFS`, and `zip.Reader`
all implement it. Supporting `io/fs.FS` directly in the loader would
give CUE users access to this entire ecosystem without the friction of
the overlay mechanism.

The original issue for this feature is at
[https://github.com/cue-lang/cue/issues/607](https://github.com/cue-lang/cue/issues/607).

[PR \#4222](https://github.com/cue-lang/cue/pull/4222) by @pskry
proposed an implementation of this feature. We are grateful for that
work and the thinking behind it. The design below builds on the same
core idea but simplifies the approach, in particular by avoiding a
synthetic path prefix and by keeping the change surface small.

## Proposal

We add two new fields to `cue/load.Config`:

```go
type Config struct {
	// ...existing fields...

	// FS, if non-nil, provides the filesystem used by the loader
	// for discovering packages, resolving modules, and reading
	// files. It is mutually exclusive with [Config.Overlay]; it is
	// an error to set both.
	//
	// When FS is nil, the loader uses the host operating system
	// filesystem ([os.Stat], [os.ReadDir], os.Open), which is the
	// default and preserves the existing behavior. Overlay can
	// be used in that case.
	//
	// When FS is set, all paths — including [Config.Dir],
	// [Config.ModuleRoot], and the arguments to [Instances] — are
	// interpreted as forward-slash-separated paths within FS.
	// Absolute paths (those starting with "/") are permitted and
	// are interpreted relative to the root of FS. Dir defaults
	// to "/" when FS is set and Dir is empty.
	//
	// FS enables loading CUE packages and modules from virtual or
	// embedded filesystems (for example, [embed.FS] or
	// [fstest.MapFS) without accessing the host filesystem.
	FS fs.FS

	// FromFSPath maps file names as they appear inside [Config.FS]
	// to file names as they should appear in error messages and
	// position information. It is ignored when FS is nil. When FS
	// is set and FromFSPath is nil, paths are left unchanged.
	FromFSPath func(path string) string
}
```

### Path semantics

When `FS` is set, all file and directory paths are slash-separated,
following `io/fs` convention. Unlike `io/fs.FS` itself, we allow paths
that start with `/`. Such paths are treated as rooted at the FS — that
is, `/foo/bar` is looked up as `foo/bar` in the FS. This preserves the
existing convention that `Config.Dir` and `Config.ModuleRoot` can be
absolute, and avoids requiring callers to reason about a fundamentally
different path model.

When `FS` is set and `Dir` is empty, it defaults to `/` rather than
the process working directory (which has no meaning relative to a
virtual filesystem).

### Mutual exclusion with Overlay

Setting both `FS` and `Overlay` is an error. The two mechanisms serve
overlapping purposes: `Overlay` patches the host filesystem while `FS`
replaces it entirely. Supporting the combination would add complexity
for a use case that `FS` alone covers. If a caller has an `io/fs.FS`
and wants to overlay additional files, they can compose FS
implementations in user code (for example, using `io/fs.Sub` or a
small merging wrapper), and this can also combine OS files with
os.DirFS.

### Error path mapping

Errors and source positions produced by the loader contain file paths.
When the underlying filesystem is virtual, these paths may not
correspond to anything meaningful on the host. The `FromFSPath`
function, when provided, gives the caller control over how paths
appear in diagnostics.

A typical use is mapping back to the original source location:

```go
cfg := &load.Config{
	FS: moduleFS,
	FromFSPath: func(p string) string {
		return filepath.Join("/real/source/root", p)
	},
}
```

When `FromFSPath` is nil and `FS` is set, paths are left as-is, which
is reasonable for testing and contexts where the slash-separated paths
are already suitable for display.

### Relation to cue/load v2

[Issue 3911](https://github.com/cue-lang/cue/issues/3911) tracks the
possibility of making a new API for cue/load. Although that API would
include support for fs.FS, the API described here is specifically made
for compatibility with the existing API and any new API would not
necessarily take this form.

### Usage example

Loading a CUE module from an embedded filesystem:

```go
//go:embed mymodule
var moduleFS embed.FS

func loadEmbeddedModule() ([]*build.Instance, error) {
	cfg := &load.Config{
		FS:         moduleFS,
		Dir:        "/mymodule",
		ModuleRoot: "/mymodule",
	}
	insts := load.Instances([]string{"."}, cfg)
	// ...
	return insts, nil
}
```

Loading from an in-memory filesystem in tests:

```go
func TestLoadFromFS(t *testing.T) {
	fs := fstest.MapFS{
		"cue.mod/module.cue": &fstest.MapFile{
			Data: []byte(`module: "example.com/test"`),
		},
		"x.cue": &fstest.MapFile{
			Data: []byte(`package x; a: 1`),
		},
	}
	cfg := &load.Config{FS: fs}
	insts := load.Instances([]string{"."}, cfg)
	if insts[0].Err != nil {
		t.Fatal(insts[0].Err)
	}
}
```

## Rationale

### Why not extend Overlay?

The overlay mechanism could in principle be extended to work without a
host filesystem backing. However, overlay was designed as a patch
layer — it assumes an underlying real filesystem and uses absolute OS
paths as keys. Bending it to serve as a standalone virtual filesystem
would require either significant internal reworking or awkward API
conventions (such as requiring synthetic absolute paths). A dedicated
`FS` field is a cleaner fit for the use case and aligns with the
broader Go ecosystem.

### Why not allow FS and Overlay together?

Supporting both simultaneously would require defining precedence rules
and reconciling two different path models (OS-absolute for overlays,
slash-separated for FS). The practical benefit is small: any layering
that a caller needs can be achieved by composing `io/fs.FS`
implementations. Keeping the two mutually exclusive simplifies both
the implementation and the mental model for users.

### Why allow absolute paths in FS mode?

The `io/fs.FS` specification requires unrooted, slash-separated paths
and does not permit a leading `/`. We relax this constraint at the
`cue/load` API level so that `Config.Dir`, `Config.ModuleRoot`, and
package arguments can continue to use absolute paths, as they do today
with the host filesystem. Internally, a leading `/` is simply stripped
before calling into the `FS`. This keeps the API transition minimal
for callers porting code from host-filesystem loading to
virtual-filesystem loading.

### Why FromFSPath rather than ToFSPath?

Errors flow outward from the loader to the caller. The mapping that
matters is from the internal FS path to the display path. A `ToFSPath`
function (mapping display paths into FS paths) would only be needed if
we accepted external paths as input, but `Config.Dir` and friends
already use FS paths directly when `FS` is set. The outward direction
is sufficient.

## Compatibility

This proposal is fully backward compatible. When `Config.FS` is nil —
the default — behavior is unchanged. The `Overlay` field continues to
work as before. The only new error condition is the case where both
`FS` and `Overlay` are set, which was previously impossible since `FS`
did not exist.

## Implementation

The implementation requires modifying the internal `fileSystem` type
in `cue/load` to route its `stat`, `readDir`, `openFile`, and `walk`
operations through the provided `io/fs.FS` when one is configured. The
overlay logic can be bypassed entirely in that case, since `FS` and
`Overlay` are mutually exclusive.

The `Config.complete` method will need adjustment: when `FS` is set,
it should default `Dir` to `"/"` instead of calling `os.Getwd`, and
path resolution should use slash-separated logic rather than
`filepath` functions.

Error wrapping and position reporting will apply `FromFSPath` (when
non-nil) at the points where file paths are embedded in `token.Pos`
values and error messages.

For now, even though symbolic links are supported since Go 1.25 via
[io/fs.ReadLinkFS](https://pkg.go.dev/io/fs#ReadLinkFS), we will not
use this interface for now. That is, interpretation of symbolic links
is left up to the underlying implementation to decide, conventionally
by treating the symbolic link as the file it points to as
[os.DirFS](https://pkg.go.dev/os#DirFS) does. This is the same
behavior used in the APIs exposed in the `mod` packages.

## Open issues

- Should `FromFSPath` be applied lazily (at error-formatting time) or
eagerly (when positions are first recorded)? Eager application is
simpler but means the original FS path is lost.
