# Supply Chain Security

Status: **Draft**

Lifecycle:  **Ideation**

Author(s): rog@cue.works

Relevant Links: [https://www.notion.so/Module-sub-proposals-fff51d3e1bdd4f98aa24a3a670c97e5e](https://www.notion.so/Module-sub-proposals-fff51d3e1bdd4f98aa24a3a670c97e5e?pvs=21)

Reviewers: mpvl@cue.works myitcv@cue.works

Approvers: name2@

Discussion Channel [Discord, GitHub, Slack, Project Management tool?]: {link}

## Overview

This document is an adjunct to the [modules proposal document](https://www.notion.so/Module-sub-proposals-fff51d3e1bdd4f98aa24a3a670c97e5e?pvs=21). In this document, we discuss security aspects of CUE modules.

## Background

As configuration is commonly used to configure crucial infrastructure, it’s clearly important to consider the security implications of CUE modules. In this document, we discuss the security aspects of CUE modules, as an adjunct to the [modules proposal document](https://www.notion.so/Module-sub-proposals-fff51d3e1bdd4f98aa24a3a670c97e5e?pvs=21). The document provides an overview of the topic, and background information on the two core aspects of module security: authorization of uploads and assurance of the contents of a module.

## Upload authorization

Upload authorization depends a lot on which registry is being uploaded to. For private registries, we anticipate that people will define their own authorization strategies. For the central registry, we will need a solution that checks that the entity uploading a module has been given authority to do so from someone that controls the namespace that the module is being uploaded to.

That solution has not been implemented yet. Instead, in the meantime, we are proposing to sidestep the question for now by relying on a github app to act as an upload agent. See [here](https://www.notion.so/Proposal-Module-publishing-Github-app-aa936fa84f2342b78a053cab407fc886?pvs=21) for a discussion of the Github app.

## Module contents assurance

For assurance of the contents of a module, we hope to leverage existing technical solutions for providing assurance of the contents of OCI images, such as [Sigstore](https://www.sigstore.dev/) and [OCI image signing](https://github.com/notaryproject/notaryproject). In general, the model will be one of ***********attestation***********: a trusted agent inspects an module when it’s uploaded to the central registry before it’s made available to the public, checks for a number of properties, and creates a signed statement (technically a *[reference manifest](https://github.com/oras-project/artifacts-spec/blob/main/manifest-referrers-api.md)*) that attests that the module has those properties. Consumers will be able to check these statements when they pull modules from a registry and gain assurance that even when they’re pulling a module from an arbitrary, potentially untrusted source, the module conforms to expectations. The `cue` command will do those checks automatically by default.

Statements that we might wish to make for a module M include, but are not limited to:

- M is the officially recognized content for $path@$version
- All dependencies within M represent the final MVS results for module resolution.
- All dependencies within M are correctly resolved to the officially recognized content for those dependencies
- The CUE code within M is syntactically valid.
- All package-local identifiers within M resolve to actual identifiers.
- All package-relative identifiers within M resolve to actual identifiers

For our initial implementation, we will not implement attestation. When fetching a module for the first time, clients will have to trust the central registry to provide the expected content. However, when downloading a module the `cue` command will store the digest (SHA256 hash) of the contents of its dependencies. This is analagous to the `go.sum` file used by Go: the difference is that `go.sum` hashes source code whereas this will hash the archives that are stored in the registry.

## Dependency confusion?

One security-related aspect warrants particular discussion: that of *dependency confusion*. This is a security issue that’s been noted in other languages: see [this article](https://medium.com/@alex.birsan/dependency-confusion-4a5d60fec610) for a good discussion of some specific cases and the overall topic.

Our proposal for CUE avoids this issue by its use of DNS-namespaced names. In particular, there is the assurance that if you don’t control a given domain (or sub-part of the domain for some special-cased multi-home domains, such as github), you cannot upload a module inside that domain.

That means that it’s straightforward for users to import their own private modules while mitigating against this attack by using *their own domain* as the import path for those modules. If someone can take over ownership of a company’s domain, the company is already in serious trouble, so this seems like it should be reasonable from a security perspective.

Note that this attack only applies against newly added or updated dependencies. For existing dependencies, the checksum file inside a module will provide assurance that only the expected module contents will be used.

DNS names can change ownership (or be hijacked), which could open up a window of opportunity for would-be causers of dependency confusion. We can mitigate against that attack by associating some secret or public key with a given domain owner. Although a new domain owner technically has control over the domain, the central registry would deny their upload because they’re unable to prove ownership of the same key. Clearly, we’d need to support key rotation too.