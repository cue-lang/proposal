# CUE modules Supply Chain Security

Status: **Draft**

Lifecycle:  **Ideation**

Author(s): rog@cue.works

Relevant Links:

Reviewers: mpvl@cue.works myitcv@cue.works

Discussion Channel: [GitHub](https://github.com/cue-lang/cue/discussions/2450)


## Overview

This document is an adjunct to the [modules proposal document](../2330-modules-v2.md).
In this document, we discuss security aspects of CUE modules.


## Background

As configuration is commonly used to configure crucial infrastructure,
it is clearly important to consider the security implications of CUE modules.
In this document, we discuss the security aspects of CUE modules,
as an adjunct to the [modules proposal document](../2330-modules-v2.md).
We identify two core aspects of module security:
authorization of uploads and assurance of the contents of a module.


## Upload authorization

Which upload authorization strategy to use depends a lot on which registry is being uploaded to.
For private registries, we anticipate that people will define their own authorization strategies.
For the central registry,
we will need a solution that checks that the entity uploading a module
has been given authority to do so from
someone that controls the namespace to which the module is being uploaded.

That solution has not been implemented yet.
Instead, in the meantime, we are proposing to sidestep the question
by relying on a GitHub app to act as an upload agent.
See [here](./2448-modules-github.md) for a discussion of the GitHub app.


## Module contents assurance

For assurance of the contents of a module,
we hope to leverage existing technical solutions regarding the contents of OCI images,
such as [Sigstore](https://www.sigstore.dev/) and [OCI image signing](https://github.com/notaryproject/notaryproject).
In general, the model will be one of _attestation_.
A trusted agent checks a module for a number of properties when it is uploaded
to the central registry and before it is made available to the public.
Upon passing, it creates a signed statement,
a *[reference manifest](https://github.com/oras-project/artifacts-spec/blob/main/manifest-referrers-api.md)*,
that attests that the module has those properties.
Consumers will be able to check these statements
when they pull modules from a registry and gain assurance that
even when they're pulling a module from an arbitrary, potentially untrusted source,
the module conforms to expectations.
The `cue` command will do those checks automatically by default.

Statements that we might wish to make for a module M include,
but are not limited to:

- M is the officially recognized content for $path@$version.
- All dependencies within M represent the final MVS results for module resolution.
- All dependencies within M are correctly resolved to the officially recognized content for those dependencies.
- The CUE code within M is syntactically valid.
- All package-local identifiers within M resolve to actual identifiers.
- All package-relative identifiers within M resolve to actual identifiers

For our initial implementation, we will not implement attestation.
When fetching a module for the first time,
clients will have to trust the central registry to provide the expected content.
However, when downloading a module, the `cue` command will store the digest (SHA256 hash) of the contents of its dependencies.
This is analagous to the `go.sum` file used by Go:
the difference is that `go.sum` hashes source code
whereas this will hash the zip archive blobs that are stored in the registry.


## Dependency confusion?

One security-related aspect warrants particular discussion:
that of *dependency confusion*.
This is a security issue that has been noted in other languages.
See [this article](https://medium.com/@alex.birsan/dependency-confusion-4a5d60fec610) for
a good discussion of some specific cases and the overall topic.

Our proposal for CUE avoids this issue by its use of DNS-namespaced names.
In particular, there is the assurance that
if you do not control a given domain, you cannot upload a module inside that domain.
For some special-cased multi-home domains, such as GitHub, an analogous
mechanism would exist for sub-parts of the domain.

That means that it is straightforward for users
to import their own private modules
while mitigating against this attack
by using *their own domain* as the import path for those modules.
If someone can take over ownership of a company's domain,
the company is already in serious trouble,
so this seems like it should be reasonable from a security perspective.

Note that this attack only applies against newly added or updated dependencies.
For existing dependencies, the checksum file inside a module
will provide assurance that
only the expected module contents will be used.

There is still the issue that DNS names can change ownership or be hijacked,
which could open up a window of opportunity
for would-be causers of dependency confusion.
We can mitigate against that attack by
associating some secret or public key
with a given domain owner.
Although a new domain owner technically has control over the domain,
the central registry would deny their upload because
they are unable to prove ownership of the same key.
As with all key-based attestation, we would need to support key rotation too.
