# Determining what files go into a CUE module

Status: **Draft**

Lifecycle:  **Ideation**

Author(s): rog@cue.works

Relevant Links:

Reviewers: mpvl@cue.works myitcv@cue.works

Discussion Channel: [GitHub](https://github.com/cue-lang/cue/discussions/3017)

## Overview

This document is an adjunct to the [modules proposal document](2939-modules.md).

This proposal discusses the issues surrounding which files
should be chosen by the `cue mod publish` command to
become part of a published module.

## Background

1) we want to avoid trying to publish files that should not be published or are not publishable. This
is [issue #2992](https://cuelang.org/issue/2992).
2) we want to avoid uploading to a registry files with content that's sensitive; specifically
if a file is ignored by git, it should not be uploaded
3) it should be possible for a registry or consumer of a module
to verify that the contents of the module correspond to the VCS commit that
the module is published from

Points 2 and 3 are potential security issues: the former to
avoid a client publishing potentially sensitive data, the latter to
make it harder for [supply-chain
attackers](https://research.swtch.com/xz-timeline) to add arbitrary
content that isn't part of the reviewed VCS source.

Note that this proposal does _not_ address module checksums ([issue
2921](https://cuelang.org/issue/2921)), although it may turn out to be
a helpful component of the solution there.


## Current behavior

The current module upload logic is summarized in the [mod/modzip documentation](https://pkg.go.dev/cuelang.org/go/mod/modzip).
It selects all files in the module directory except:

- VCS directories (`.bzr`, `.git`, `.hg`, `.svn`)
- CUE submodule directories (`cue.mod`)
- any non-regular file (symbolic links, for example)
- empty directories or directories containing only excluded files

## Proposal

We propose that the existing behavior be kept as is, except that the
set of files considered can be restricted according to a new `source`
field added within `cue.mod/module.cue` This specifies the mapping
from a module's "source" (for example the VCS commit where the module
is developed) to the files that are considered end up the actual
module in a registry. Initially, this will have only a single
subfield, `kind`.

That is, the set of files implied by `source` will feed into the
existing algorithm used by
[modzip](https://pkg.go.dev/cuelang.org/go/mod/modzip), acting as a
filter on the files considered by that logic. Note that that algorithm
itself may change over time. If it does, we can use the
`language.version` field to choose which version of the
algorithm to use, making the choice unambiguous.

Note that defining `source` as a struct means that it will be
easy to add other source-related fields that influence the
choice of files within a module. As an example, we might wish
to implement a mode that excludes all files other than those
directly relevant to CUE evaluation.

```cue
// source holds information about the source of the files within the
// module. This field is mandatory at publish time.
source?: #Source

// #Source describes a source of truth for a module's content.
#Source: {
	// kind specifies the kind of source.
	//
	// The special value "self" signifies that
	// the module that is standalone,
	// associated with no particular source other than
	// the contents of the module itself.
	kind!: "self" | "git"

	// TODO support for other VCSs:
	// kind!: "self" | "git" | "bzr" | "hg" | "svn"
}
```

If the `source` field is present and `source.kind` is `self` the
behavior will be exactly as it is currently: the files present in the
module directory at the time of publishing will be those chosen as the
initial file set.

When the `source field is present and `source.kind` is not `self`, it
usually names a VCS; `cue mod publish` will include only files that
are considered by that VCS to be part of the current commit. It will
also fail if the current directory is not clean with respect to the
current commit or if information on that VCS cannot be found. There
might be command line options to tell `cue mod publish` to ignore
those errors.

When the `source` field is not present, `cue mod publish` will fail.

## Possible default behavior

As specified above, when there is no `source` explicitly specified,
the `cue mod publish` command will fail, because there is no default
behavior in that case.

It is possible that a default value could be chosen based on local
context, specifically whether the `cue` command detects a VCS holding
the module's directory.

This default behavior would result in a failure if multiple VCSs are
present, requiring the user to disambiguate by setting an explicit
`source.kind` value.

The introduction of this behavior in its precise semantics will be
guided by real world experience with the implementation.

## VCS metadata

`cue mod publish` will include VCS metadata about the commit that's
associated with the uploaded version and the directory of the module
within that commit. The VCS chosen is determined by the `source.kind` field as
described above.

This metadata can be used by the registry and consumers to
verify that the uploaded content of the module does indeed correspond
to the advertised VCS commit (see [Verification](#verification)
below).

The above implies that single VCS's metadata is published
with a module and that if `source.kind` is `self`, there will be no metadata
published, even if there is a VCS directory present. This seems
consistent, but it would also be possible to publish auxiliary metadata
about VCS commits if that were deemed useful, although that
would not be deemed "canonical" information on the source in the same way.


## Source-of-truth validation

It should be possible for a registry or a module consumer to verify
that a module contains exactly the files that were part of the
VCS commit that's part of the module's metadata. This
verifiability plays an important role in avoiding supply chain
attacks.

In order to verify a module, a registry or consumer needs access to:

- the module itself;
- VCS metadata describing the kind of VCS used as a source
of truth for the module, the commit the module was taken from,
and the directory within the VCS directory containing the module;
- a location for where to access the VCS source.

The first two will both be part of the data uploaded by `cue mod publish`.
The location will not, because it can potentially change
and/or have several possible entries.

For some classes of module (for example modules rooted under
`github.com`), the location might be well defined; for others
(for example modules with "vanity paths"), the location might
be specified as metadata retrieved from a `/.well-known/...` URL,
or configured as part of the registry itself. These conventions are beyond
the scope of this document and will be covered by a future proposal.

In any case, once the information is known, a registry receiving
a module upload or a client consuming a module can verify
the contents by applying the following steps:

1) download VCS contents of the module's commit from the source location
1) change into the module's directory within that
1) run the same logic as `cue mod publish` but producing only the
upload artifact (zip file), not actually uploading it
1) verify that the contents of the zip file match the contents of the
uploaded module

An alternative approach for CUE registries (not stock OCI registries)
might be to provide an alternative upload API that allows the
client to point the server at the commit to be published
and get it to generate the upload itself without the need for
the client to transfer the data.
