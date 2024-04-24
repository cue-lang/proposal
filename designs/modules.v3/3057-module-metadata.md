# Proposal: CUE module metadata

Status: **Draft**

Lifecycle:  **Proposed**

Author(s): rog@cue.works

Relevant Links:

Reviewers: mpvl@cue.works myitcv@cue.works

Discussion Channel: [GitHub](https://github.com/cue-lang/cue/discussions/3057)


## Abstract

This document is an adjunct to the [modules proposal document](2939-modules.md).

We wish to attach VCS and other metadata to a module when uploading it
to a registry. This information can indicate specifics of where a
module was derived from, for example the commit hash of the VCS commit
from which the module was derived.

This document proposes a way to include this information in the
OCI manifest uploaded to a registry.

## OCI manifests

The OCI manifest schema provides an `annotations` field that is designed
for metadata. It's a simple one level map of string to string.

It is described [here](https://github.com/opencontainers/image-spec/blob/e2edbc8c1723063d5167ea6b420eaebbc74b552c/annotations.md).

To quote some relevant points from that document:

- Annotations MUST be a key-value map where both the key and value MUST be strings.
- While the value MUST be present, it MAY be an empty string.
- Keys MUST be unique within this map, and best practice is to namespace the keys.
- Keys SHOULD be named using a reverse domain notation - e.g. `com.example.myKey`.
- Consumers MUST NOT generate an error if they encounter an unknown annotation key.

## Proposal

We propose that CUE-related metadata be stored in a set of keys
prefixed with `org.cuelang.`, i.e. namespaced within the CUE project.
To hold to the spirit of the manifest annotations, we will stick to a
one-field-one-value convention rather than (for example) encoding all the
metadata as a single JSON blob holding all the CUE-related fields.

As these fields are visible in isolation within third party OCI registry
browsers, it arguably makes sense to describe the fields individually
rather than considering them as a whole. This contrasts with the
approach taken by the `module.cue` file where the entire file
can be considered solely in the context of the language version.

The following fields are initially proposed:

- `org.cuelang.vcs-type`: the kind of VCS for which VCS metadata is supplied (initially
supporting only `git`).
- `org.cuelang.vcs-commit`: the commit associated with the module's contents
- `org.cuelang.vcs-commit-time`: the time of the above commit, in RFC3339 syntax

Note that we are _not_ proposing that the remote location of the VCS
is included, as that is not something that can be used or checked by a
CUE registry. We are free to change that decision in the future if
such a field might prove to be useful.

For example:

```
annotations: {
	"org.cuelang.vcs-type": "git"
	"org.cuelang.vcs-commit": "2ff5afa7cda41bf030654ab03caeba3fadf241ae"
	"org.cuelang.vcs-commit-time": "2024-04-23T14:48:10.542193563Z"
}
```

## Backward and forwards compatibility

When creating a module, the CUE tooling will add all the metadata
fields that it knows about.

The consumer of a CUE module will ignore any fields that it does not
know about (including fields with the `org.cuelang.` prefix. This
conforms to the "Consumers MUST NOT generate an error if they
encounter an unknown annotation key" requirement.

The CUE project will be careful to add fields only in a backwardly
compatible manner and avoid changing the meaning of existing fields.

When processing uploads, a CUE registry may inspect the language
version and reject uploads that don't contain all the expected
metadata fields for such a version. It may also reject uploads with a
version that is not within acceptable bounds. This logic, however,
will be present in the registry code but _not_ the code to actually
parse the metadata, which will parse any fields it knows about and
ignore fields it doesn't.
