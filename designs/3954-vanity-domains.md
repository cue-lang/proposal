# Proposal: Support for Vanity Domains in the CUE Central Registry

## Objective

The objective of this proposal is to extend the functionality of the CUE Central Registry to support "vanity domains" (domain names chosen by the user) as an alternative to the current GitHub-based namespaces. This would enable users to register and manage modules under domain-based namespaces, provided they can demonstrate control over the respective domain. This functionality aims to provide flexibility, scalability, and a more decentralized approach to module management.

## Background

The CUE Labs' Central Registry exists as a well-known place for well-known schemas for well-known services/software. It is the default registry used by `cmd/cue`. CUE modules have a module path, a domain-name-based identifier similar to Go modules. By contrast with the Go proxy the Central Registry is, as its name suggests, is a registry that modules are published to directly.

When publishing to the Central Registry, the module author needs to demonstrate control of the namespace implied by the prefix of the module path. The Central Registry is not an identity provider. Instead for now we leverage GitHub as an identity provider, with a view to adding other authentication providers in the future. As such for now a CUE Labs identity is tied solely to a GitHub identity. A further restriction is that modules published to the Central Registry must exist in the `github.com` repository namespace because that makes it easy to know who should have access to a given part of the namespace.

When publishing a module, a user of the Central Registry must be able to demonstrate control over the relevant part of that `github.com` namespace. For example, if user `alice` attempts to publish module `github.com/alpha/beta`, then the Central Registry requires that `alice` has write access to the repository `github.com/alpha/beta`. Read access for private modules is determined in the same way.

In summary currently GitHub is leveraged for both authentication and authorization, and modules can only be published within the `github.com` namespace.

This design effectively centralizes control under GitHub, which limits the ability of organizations, projects, or individuals to leverage other naming schemes. Supporting vanity domains would allow users to publish CUE modules under their own hostnames (e.g., `example.com/my-module`) by demonstrating ownership of the domain.

## Proposal

### Design Overview

A common and widely accepted mechanism for verifying ownership and configuration for domain-based namespaces is the `.well-known` convention ([RFC 8615](https://www.rfc-editor.org/rfc/rfc8615)). This standard provides a way to host metadata files under a predictable path (`https://example.com/.well-known`) to establish identity or configuration. Leveraging this convention ensures interoperability, avoids cluttering domain root directories, and aligns with best practices for web-based configuration.

The proposed design introduces a mechanism to verify domain ownership and define authorization rules for publishing modules. This will be achieved using a `.well-known/cue-central-registry.json` file hosted at the domain. The registry will use this file to determine permissions and manage module namespaces under the domain.

The `.well-known/cue-central-registry.json` file must define:

1. **Authorization Rules**: These rules specify who can publish or access modules under the domain. Initially, this will support GitHub-based authorization, where a GitHub repository serves as a proxy for ownership.
2. **Refresh Duration**: A hint for how long the registry should cache the verification data.

### File Specification

A simple straw-man schema for this `.well-known/cue-central-registry.json` file is as follows:

```
#WellKnown: {
	auth!: {
		type!: "github"

		// repo is used as a proxy for the authorization to
		// the domain. Anyone that can push to this repo
		// can push to the domain in the Central Registry;
		// anyone that can read from this repo can read modules
		// from the domain in the Central Registry.
		repo!: =~"^[^/]+/[^/]+$"
	}
	// cacheDuration specifies the maximum duration
	// for which the contents of this file should be considered
	// to be valid before refreshing. This is only a hint:
	// a client might choose to round up to a longer time
	// to avoid excess traffic.
	cacheDuration?: time.Duration
}
```

Under this approach, when the Central Registry detects an operation on a module in the namespace `$HOST/...`, where `$HOST` is not a special-cased host such as `github.com`, it fetches and parses the `.well-known/cue-central-registry.json` file at `https://$HOST/.well-known/cue-central-registry.json`. The rules in this file govern who can publish or access the module. By using JSON format, we make it clear that the contents are concrete, and it's also more accessible to non-CUE tooling to either produce the file or parse it.
As an exception to the above check, if a module has been published under a vanity domain and the associated repository is public-access when published, this creates a *public module* and as such we will *not* subsequently check the `.well-known` file for read requests on that module. This allows us to use an efficient access path for public modules. Note that in general 	we forbid making a public module private because that's as problematic as removing a module.

### Extensibility for Future Enhancements

This design can be made more general to support finer-grained access control in the future. For example:

- **Namespace-based Permissions**: Allowing specific rules for sub-paths within the domain namespace.
- **Support for Non-GitHub Authorization**: Future extensibility to other authorization systems (for example an RBAC scheme defined by the Central Registry itself).

An example of a more general file format that allows authorization requirements to vary in different parts of the namespace:

```
#WellKnown: {
	// auth holds authorization for the entire namespace.
	auth?: #Auth

	// namespace holds authorization for parts of the namespace.
	// Each prefix key applies authorization to all modules
	// with that path prefix within the host. If two prefixes
	// apply to the same module, the deepest of the two applies.
	namespace?: [prefix= =~"^(/[^/]+)+$"]: #Auth

	// refreshDuration specifies the maximum duration
	// for which the contents of this file should be considered
	// to be valid before refreshing. This is only a hint:
	// a client might choose to round up to a longer time
	// to avoid excess traffic or round down to a shorter time
	// to prevent excessive cache lifetimes.
	refreshDuration?: time.Duration
}

#Auth: #GithubAuth

#GithubAuth: {
	type!: "github"

	// repo is used as a proxy for the authorization to
	// the domain. Anyone that can push to this repo
	// can push to the domain in the Central Registry;
	// anyone that can read from this repo can read modules
	// from the domain in the Central Registry.
	repo!: =~"^[^/]+/[^/]+$"
}
```

One other possible enhancement might be to make it possible to host the same file content from different hosts, making it trivial to set up a static web server serving identical content to many subdomains. In that case, we could potentially provide an entry that maps host to namespace:

```
// hosts holds host-specific auth information,
// mapping a given hostname to its namespace
// auth information. If this is present, then namespace
// auth are ignored. When a given host h is consulted
// in this case, the auth namespace is defined by hosts[h].
hosts?: [hostname=string]: [prefix= !=""]: #Auth
```

Again, this would be a backwardly compatible enhancement.

Another possible enhancement would be to have provision for explicitly delegating authorization for a particular part of the namespace to some other authority (for example a `cue-central-registry.json` file at another path). This could be done by defining a different kind of authorization.

```
#DelegatedAuth: {    type!: "delegate"

    // configURL specifies the URL of a configuration file in the same
    // format as cue-central-registry.json that will be used for
    // auth decisions about this part of the namespace.
    configURL!: string
}

#Auth: #GithubAuth | #DelegatedAuth
```

## Verification of module source

There is a [provision in a module.cue file](https://cuelang.org/docs/reference/modules/#determining-zip-file-contents) for specifying the kind of VCS source, and in the module's metadata for specifying the VCS commit that holds the source. When publishing to a [github.com/x/y](http://github.com/x/y) repository, when the VCS source is specified as `git`, a registry can check that the VCS commit is present in that repository and has the expected contents. This approach can also apply when publishing to a vanity domain: by stripping the vanity domain prefix from the module path and prepending it with the auth repository name, we can then apply exactly the same style of check for vanity domains as we can for regular github repositories. In the future, when alternative authorization mechanisms can be specified, it would be possible for the `.well-known` file to define the source repository as well as the authorization criteria: the two do not necessarily have to be linked.

## Alternatives Considered

### Subdomain support

Supporting sub-domains introduces additional complexity and potential risks (e.g., abuse of wildcard namespaces). For simplicity and security, this proposal deliberately excludes such support at this stage.

### Alternatives to `.well-known`

Other mechanisms for domain verification, such as DNS TXT records or custom endpoints, were considered. The proposed solution does not rule out these methods, but the `.well-known` convention seems like the easiest thing to start with.

The Go ecosystem uses a different mechanism: it makes an HTTP request directly to the full path of a given module, for example `GET` [`https://golang.org/x/mod?go-get=1`](https://golang.org/x/mod?go-get=1) and looks for a Go-specific `<meta>` tag in the resulting page. This is appropriate for Go, where modules names point directly to where they are hosted, but does not seem appropriate for CUE where a location is merely *implying* access rights to a namespace in the Central Registry. Other than on a case-by-case basis (for well known content-hosting sites, for example), it does not seem inherently safe to assume that because a piece of content exists at a specific location, the creator of the content should have freedom to publish modules to the Central Registry in that name space.

Here is a more detailed discussion of why I don't see the above possibility as appropriate for CUE:

When people see our host-prefixed module paths, they tend to assume that there is a direct correspondence between those paths and the content in them. For example, they assume that given some module path `foo.com/bar`, the source of truth for its contents will be somewhere like `https://foo.com/bar`.

However, by design that's not the case with CUE: the contents are stored in a registry and in the general case the path is only used as a way to leverage DNS as a way for users to prove that they have ownership over that part of the namespace. Although users can explicitly decide to associate the uploaded modules with a VCS source (via the `source` field), that association is not a general rule and thus we should not be relying on it in all cases.

 Using the `.well-known` convention is explicit acknowledgement of that separation of concerns: that convention is commonly used as a way of demonstrating proof of ownership/control of a given host name. Thus this convention is, I believe, at least *sufficient* and secure as a way for users to be able to let the Central Registry know auth requirements for a given host.

But maybe this requirement is too onerous: perhaps we wish to allow anyone to publish to a namespace, not necessarily at the top level, in the Central Registry if they have control over that portion of the namespace. This is the approach taken by the Go ecosystem: the information for where to read module data for a given module `$M` is determined by reading `https://$M?go-get=1`.

If we took that approach, it would mean that the module namespace for a site would need to map directly to the actual hosted namespace, tying the two namespaces more closely together. The URL namespace would not just prove ownership, but define the actual content structure within the registry. To me, that doesn't seem quite the right direction to be going: it makes the separation of concerns between module content and namespace control less clear and adds to the potential for confusion.

It also makes the Central Registry implementation more complex: defining auth for a registry module has more to it than defining auth for Go modules:

- the registry itself has to authorize and store content for new module versions, rather than just read existing data.
- administration might be harder: for example, a single top level `.well-known` file could act as a single place to define admin-level configuration, but that's harder if there is no single top-level configuration for a host.

The former in particular means that every time a new module is pushed, the registry will need to do one or two HTTP GET requests to the namespace in question to determine auth requirements for that module.

While it *might* be OK, in general this approach seems less "obviously correct" to me than the single-level `.well-known` approach, involving more design, review and implementation work than seems necessary at this time. It might well be possible to use the Go-like approach for CUE while maintaining good security, but the implied namespace-control semantics of doing that seem potentially troublesome; to repeat from above: it does not seem inherently safe to assume that because a piece of content exists at a specific location, the creator of the content should have freedom to publish modules to the Central Registry in that name space

Also, the `.well-known` approach does not preclude implementing a more general path-based scheme in the future. If we wished to allow it, we could simply implement such a scheme, for example by adding a flag to `.well-known` that explicitly enables that feature for a given host.

Given its "more obviously correct" security, reduced implementation cost, and the fact that choosing this approach does not seem to back us into a corner, I suggest that the `.well-known` approach is preferable.

## Cross-Cutting Concerns

1. **Security**: Proper validation of the `.well-known/cue-central-registry.json` file is critical to prevent spoofing or unauthorized access.
2. **Caching and Performance**: To minimize unnecessary traffic, the registry should respect the `refreshDuration` hint while implementing reasonable caching strategies.
3. **Backward Compatibility**: The existing GitHub-based system will remain fully supported and coexist with the new domain-based namespaces.

## Conclusion

This proposal introduces support for vanity domains in the CUE Central Registry using the `.well-known` convention. The approach is straightforward, aligns with web standards, and lays the foundation for more advanced features in the future. By implementing this, the Central Registry can evolve into a more flexible and decentralized platform while maintaining security and simplicity.
