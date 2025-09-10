# **cue cmd v2: cue cmd analysis**

*Document version: v1*<br>
*Author: Paul Jolly ([myitcv](https://github.com/myitcv))*

For many users, `cue cmd` is where CUE really "clicks": declarative workflows
that act as the perfect glue between data, logic, control flow, and code.

We’ve been gathering feedback, bugs, and feature requests for a potential v2 of
`cue cmd` in issue [#1325](https://github.com/cue-lang/cue/issues/1325) and
linked issues/discussions.

A big thank you to everyone who has already contributed thus far.

We are now in the process of synthesising that feedback, and this document is
designed to capture the feedback and analysis of `cue cmd`:

* If you use `cue cmd`, what works well for you?
* What feels awkward, missing, or limiting?
* What would make `cue cmd` a better fit for your real-world use cases?
* If you can't use `cue cmd` for some reason, why? What are the issues, gaps

This document, despite living in the
[cue-lang/proposal](https://github.com/cue-lang/proposal) repository, is not a
proposal. Instead, it is an analysis of  all the feedback bug reports etc in and linked from
[#1325](https://github.com/cue-lang/cue/issues/1325). This document will be
updated as the definitive "source of truth" for the analysis of `cue cmd`, with
version bumps reflecting logical "cuts" of changes/additions etc.

Please provide comments on the analysis in
[#4081](https://github.com/cue-lang/cue/discussions/4081).

This document does not seek to prejudge the answer to the question "should `cue
cmd` v2 even exist?" Hence the section below on the scope of `cue cmd` v2 is
intentionally left blank for initial reviews of this document, allowing for
public discourse on this point in particular. Analysis of bugs/shortcomings etc
with `cue cmd` can be seen as more objectively analysis of a thing that already
exists.

One thing to also stress is that this document seeks to entirely avoid talking
about designs of `cue cmd` v2, except where pointing out design decisions of
`cue cmd` are good/bad, or referring to the design/API of other "systems" where
that is a critical interface of such a declarative workflow setup.

Please excuse the bullet point form of this document. Indeed please also excuse
any overlap between points. There has been a significant amount of feedback to
process to produce this initial draft. As such, we have focussed on trying to
capture the base "facts" accurately, and then finesse the document (as
required).

## A brief history

* `cue cmd` has been present in CUE from the very early days
* `cue cmd` provides a means by which users can declare workflows
* Workflows are declared as named commands composed of tasks
* Tasks are built on top of standard library primitives that live beneath
  [https://pkg.go.dev/cuelang.org/go/pkg/tool](https://pkg.go.dev/cuelang.org/go/pkg/tool)
* A user invokes a workflow by calling a named command, e.g. `myworkflow`, like
  `cue cmd myworkflow`

## How `cue cmd` works currently

* Workflows are declared as commands which are composed of a set of one or more
  tasks. For the purposes of this analysis, we use the term workflow and command
  interchangeably. But to be clear, a workflow is a thing that `cue cmd` has
  implemented via named commands. So the former is the more general term.
* Dependencies between tasks within a command might be declared via references.
  Such references represent a dependency on the data/state of one task. e.g. the
  output of one task (running a CLI command) might become the input to another
  (unifying with some CUE configuration and writing the result to a file).
* The dependency graph defines the order of execution of tasks
* Workflows are declared in `*_tool.cue` files that belong to a package
* Running a command (workflow) involves specifying the name of the command and
  the set of instances over which to run that command, e.g. `cue cmd myworkflow
  ./...`. These instances must all belong to the same package, and the workflow
  must be declared in a `_tool.cue` file in that package.
* If no argument is provided beyond the command name, it is assumed that command
  is defined in `.` (which might need to be refined via a package qualifier)
* Tasks can be placed at an arbitrary depth within a command declaration struct,
  but sub tasks (declaring a task within the structure of another task) are not
  permitted.
* Tasks cannot be declared in lists.
* Tasks are identified by `$id` fields that are "special" to the
  [https://pkg.go.dev/cuelang.org/go/pkg/tool](https://pkg.go.dev/cuelang.org/go/pkg/tool)
  hierarchy (which also more formally specifies the structure of commands and
  tasks)
* It is currently possible to use a task via its `$id` "via the backdoor" by
  simply declaring a task and setting an `$id` field, without explicitly
  unifying with the `tool/$pkg` task.
* `CUE_DEBUG=toolsflow` allows for a very primitive form of visualisation of the
  workflow
* Non-package/instance arguments must be specified via
  [injection](https://cuelang.org/docs/reference/command/cue-help-injection/)
* Manual dependencies between tasks can be declared via `$after` where
  references otherwise wouldn't exist
* Concurrency of task execution is unbounded and limited only by the dependency
  graph between tasks
* There is an overlay of the workflow on top of the package it is working on.
  This allows the workflow to reference data/etc outside of the command
  namespace within which the workflow and tasks are declared. This includes
  hidden fields in the same package.

## What should the scope of cue cmd v2 be?

* Does a focus on `cue cmd` v2 distract from efforts to make CUE more natively
  available in other languages? [cue-lang/cue#1325
  (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-3225238367)
* *(intentionally left largely blank!)*
* ...

## How/where are workflows and tasks declared?

* Other points pick up the fact that requiring the workflow to be part of the
  package to which the instances belong is constraining
* The `command` namespace also seems to place an awkward merge/overlay
  relationship between those instance "arguments" and the workflows
* As an example of a different setup, Go has `_test.go` files that when declared
  as part of an external test behave like a different package to the package
  under test. This clear package-based separation removes any questions of
  merge/overlay, but does then limit the interactions to exported identifiers.
  Right now, workflows can interact with hidden fields in the package.
* See related points about the inputs to a workflow, and their relationship to
  the workflow
* Whether to move away from the model whereby `$id`-style fields are used to
  identify tasks, for example towards a model where a hidden field from within a
  `tool/*` package is used to identify a task. This is very much related to
  where workflows/commands/tasks are declared in the first place, but also to
  the question of how we make workflows/commands/tasks reusable
  * The experience of `hof` [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-3215728955)
    and the idea of moving away from `_tool.cue` files is related here
* Related to the points regarding dependencies and control flow, is the idea
  that we should support lists of tasks: [Notes on `cue cmd` · cue-lang/cue ·
  Discussion #3917](https://github.com/cue-lang/cue/discussions/3917) (point
  15). Where it is known and required that steps happen in sequence, providing a
  list is cleaner than a struct where we have to find useful names for each
  step. However we need to be somewhat careful in doing this, because lists
  critically hurt composability. Items in a list do not have names, so it's much
  more difficult to use/work with a list, let alone unify other
  constraints/tasks. Not only that, with a clean/fluent means of specifying
  dependencies between tasks in a struct based setup, the only "need" for lists
  is that it's less cumbersome to declare initially. Almost all other
  workflow/tasks systems that are imperative are list/sequence based, so this
  has good precedent and understanding. But given the lack of composability,
  it's not without its flaws: i.e. the ongoing maintenance of a list based
  approach might well outweigh the initial saving derived from not having name
  steps.
* Currently it is possible for a task to be declared/specified by including a
  struct value with an appropriately set `$id` field. Whilst doing so works, it
  mean that default in the task's declaration are not applied, and accordingly
  the Go implementation has to maintain a "duplicate" of this default logic.
  e.g. [this
  default](https://github.com/cue-lang/cue/blob/fac9a305d0db4c43ce358551232eee60f2dfe3dc/pkg/tool/http/http.cue#L30)
  is "duplicated" in [this Go
  code](https://github.com/cue-lang/cue/blob/fac9a305d0db4c43ce358551232eee60f2dfe3dc/pkg/tool/http/http.go#L76).
  We should not allow this kind of back door task injection, and instead design
  an approach which requires explicit unification with the task declaration.
* Should we require that tasks to be contained by a command that indirectly
  invokes them? This has overlap with the question of re-use of workflows/tasks.
  The model of referenced tasks becoming dependents can lead to things being
  brittle because it's arguably too easy to end up with two tasks when you
  intend to only have one, which means the synchronisation/coordination point is
  lost::

```
exec cue cmd x
cmp stdout stdout.golden

-- cue.mod/module.cue --
module: "cue.example"
language: {
	version: "v0.15.0"
}
-- p/p.cue --
package p

import (
	"tool/exec"
)

metadata: exec.Run & {
	cmd: ["ls"]
}
-- x_tool.cue --
package x

import (
	"tool/cli"
)

helperFlow: {
	echo: cli.Print & {
		text: "hello"
	}
}
command: x: helperFlow & {
	print: helperFlow.echo
}
-- stdout.golden --
hello
hello

```

* What interaction is there (if any) between the package defining a workflow and
  the packages it (the workflow/task) might need to load/analyse etc? If we move
  to a model of the instances/packages being arguments/inputs to a workflow (as
  opposed to the existing "overlay") what changes as far as access to hidden
  fields are concerned? Example:

```
exec cue cmd print
cmp stdout stdout.golden

-- cue.mod/module.cue --
module: "cue.example"
language: {
	version: "v0.15.0"
}
-- x.cue --
package x

_x: 5
-- x_tool.cue --
package x

import (
	"tool/cli"
)

command: print: cli.Print & {
	text: "_x: \(_x)"
}
-- stdout.golden --
_x: 5
```

## Workflows and task reuse

* We want the ability to publish workflows and tasks as part of modules so that
  they can be reused, especially to the [Central
  Registry](https://registry.cue.works/) and OCI registries generally. i.e.
  these things are dependencies like regular CUE
* We need the ability to trigger one workflow from within another, not just
  re-use tasks. This implies some sort of symmetry between the ways that
  workflows and tasks are specified in terms of inputs and outputs
* We want the ability to specify where the workflow that should be run is
  declared
  * Right now it must be part of the same package as the instances passed as
    arguments
  * Should also support `$pkg@$version` remote form like `go run $pkg@$version`
    - i.e. ignore main module (if there even is one)
  * For example the workflow could be a package in a different directory/module.
    Hence we could have something like `-C` with `cmd/go`, `tar` etc, where the
    `-C` is used to specify a different context for the workflow (which could be
    a different module)
  * More specifically, the workflow and the inputs don't always belong to the
    same package (this is currently a requirement)
* The concept of tasks and workflows (whatever that split might end up being)
  being defined on top of primitives in the standard library seems sound. It is
  a good separation of concerns, and allows for "layers" or "abstractions" to be
  combined through the familiar mechanism of composition in CUE. For example,
  have the concept of the abstract
  [`exec.Run`](https://pkg.go.dev/cuelang.org/go/pkg/tool/exec) in the tooling
  standard library, allows someone to write and publish a task that builds on
  that primitive to provide an abstraction for the `ls` CLI. That abstraction
  need not indeed should not live in the standard library; instead it should be
  published into the module ecosystem for reuse by others (with the abstraction
  ideally being maintained as close to the thing it is wrapping as possible).
  Such reuse avoids the need for the N users of `ls` to each write their own
  abstraction. And much like tooling/services/etc beyond plain CLI commands, it
  encourages a schema-first, structured approach to defining interactions
  between "things", something that is good for both humans and machines.
* Related to the previous point is the question of what task and workflow
  primitives should exist in a "standard library" or equivalent? The existing
  [`pkg/tool/...`](https://pkg.go.dev/cuelang.org/go/pkg/tool) hierarchy fairly
  closely follows the Go standard library, and leverages the fact that it is
  very much cross platform. This generally feels like a good primitive or
  building block. Whilst there are a number of "missing" bits of that standard
  library (many issues on this subject), it feels broadly like the "right
  direction".

## What configuration/inputs does a workflow "operate" on?

* Somewhat orthogonal to `cue cmd` v2 specifically (and with some crossover to
  the notion of `@export`), but the `./...` pattern can at times be a bit
  awkward. In the k8s world, this pattern is commonly used to mean "the leaf
  instances", a pattern that works if ancestor directories to the root do not
  declare any values that are iterated over by tools and the like. Perhaps what
  tools are looking for in this situation is a way of working with "the exported
  configuration(s)"?
  * This is somewhat picked up in [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-1671629204)
    which talks about the confusion of "on what packages/instances is this
    command run?"
  * Very much related to this point is that `cue export ./...` does something
    quite different to what `cue cmd mytask ./...` does. The former performs the
    export action on every instance matched by `./...` whereas the latter runs a
    workflow once for the combined result of the single instance derived from
    unifying all instances in the same package matched by `./...`. This
    asymmetry is extremely confusing to users, and the cause of countless "what
    is this command going to run on" questions. See [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-1697746105)
    for an example.
* We need a clear explanation of how a package declaring a workflow does/doesn't
  interact with any package/instance arguments. At the moment there is a rather
  complicated "merge"/"overlay".
  * This is very much related to "where" commands get declared: in a Go-style
    `_test.go` space?
  * Workflow declaration and "data" need not be in the same package (see above)
    - but where they are, does that allow hidden field access?
* We need a good UX for the different inputs to running a workflow:
  * The name of the workflow
  * Where the workflow defined
  * Arguments
    * The CUE package/instance inputs to be "worked on" (if there are any)
    * Other arguments (examples needed here)
  * Related specifically to the point about arguments, how should a
    workflow/command declare the API/flags that it can be driven by? This feels
    like an integral part of not only using such a workflow but also
    automatically documenting it, i.e. `cue cmd mycmd --help`.
    [cue-lang/cue#3357](https://github.com/cue-lang/cue/issues/3357) is related
    in this regard
* In case a workflow is being run by another workflow, as opposed to "directly"
  via arguments passed on the command line via `cue cmd` or similar, we will
  need the ability to say "run this workflow and here are your CUE/other
  inputs". Again, this seems to point to some symmetry between how tasks and
  workflows are specified.
* A bullet point in
  [cue-lang/cue#1325](https://github.com/cue-lang/cue/issues/1325) talks about
  providing a means by which a workflow can load data via `@embed`. A more
  general point is actually that a workflow task might well want to load some
  CUE/other files via the same mechanism that `cmd/cue` itself supports for a
  command like `cue export`. This point is closely related to the fact that we
  might want to make more explicit the loading of "input" CUE for processing in
  a workflow (with some connection to the arguments provided on the command
  line) vs today's implicit loading (it's part of the same package).
  * In various exchanges, an explicit "load" task (built on top of
    [`cue/load`](https://pkg.go.dev/cuelang.org/go/cue/load)) has been
    suggested to make very explicit how inputs are handled within a workflow
    (and by extension, task)
  * Indeed could the various `cmd/cue` commands (e.g. export) be written in
    terms of `cue cmd` v2? (from various

## Dependencies between workflows and tasks, and control flow

* Ensuring that we have clear guidelines and the relevant builtins for people to
  conditionally depend on task output.
  [cue-lang/cue#1593](https://github.com/cue-lang/cue/issues/1593) is an
  example of where a user is prompted for input. If that input is a long string
  thing 1 happens, otherwise thing 2 happens. The "trick" here is that the test
  of "is the user response long or short" is something that is an error until
  they have responded. We need to clearly articulate to folks how to handle
  this, and make sure the result is clear and obvious. This might be via
  something like checking "`if taskComplete(a) {... }`" or more CUE-style checks
  on "`if isConcrete(A.response) {...}`"
* Right now, we have a rather uneven approach to the notion of what it means for
  a task to fail, and whether that task failing is fatal for the workflow. For
  example, it's possible (and indeed desirable) to have a call to `exec.Run`
  fail, and have other tasks depend on whether that succeeded or failed. But
  this does not extend, for example, to `file.Create`. If that task fails, then
  the entire workflow is stopped immediately (even if there is another "thread"
  of task execution not dependent on that failure, that could otherwise
  continue). This gets very close to the question/notion of handling failures.
  Here we should look towards things like `set -e` in bash, the notions of how
  GitHub actions allows "continue on error" etc. We need to think about a safe
  default (which arguably the current behaviour is) with opt-ins to allow people
  to relax.
  * Per [cue-lang/cue#1568
    (comment)](https://github.com/cue-lang/cue/issues/1568#issuecomment-1200371475)
    this is also very related to the concept of declaring dependencies between
    tasks, something that we currently do via `$after` in a limited form of
    dependency declaration. It might be that more formally introducing the
    concept of failure for all tasks means we can trivially and naturally do
    away with the need for `$after` because one task can then depend on another
    by referencing its "exit code" or equivalent
  * This further leads to the observation that we don't necessarily need/want a
    `set -e` global equivalent like bash, although noting that in some cases
    this is what the user will want (squinting really hard, this could be
    achieved by unifying every task with a "must succeed" value, although with
    arbitrarily nested tasks (and tasks in dependencies, it's hard to see this
    kind of approach working).
  * It's also useful to note that one task depending on another's "exit code" is
    another model by which we can introduce partial error handling behaviour.
    But critical to this is that a unification failure "exit code was 1,
    expected 0" should not cause the entire workflow to fail if there is still
    other work that can proceed. i.e. we need to be clear how users can not only
    depend on a task completing (whether success or failure), and also be clear
    how they can further constrain the success/failure of a task, e.g.
    `must(userinput.exitCode, 0)` (in total pseudo code)
  * Related to this is
    [cue-lang/cue#2324](https://github.com/cue-lang/cue/issues/2324) which
    explores the idea of whether sub tasks are something we should support as a
    means of expressing existence-dependency relationships between tasks.
  * [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-1494634067)
    and [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-2192457001)
    (specifically the point "It would be nice to pause execution...") reference
    a class of workflows that involve starting a server, waiting for that server
    to be ready, then running a series of tests, (then stopping the server). It
    seems likely that we need/want to support such a class of task. Which
    refines this point of "wait until task X is complete" to "wait until task X
    is ready". And it necessitates some kind of "shutdown" or "done" signalling
    task. As well as clear best practices/advice on how to signal "ready" and
    "stop". [cue-lang/cue#3774](https://github.com/cue-lang/cue/issues/3774)
    talks about exposing `cue serve` which is somewhat related.
  * From the various threads of conversation on errors, whether they are fatal
    etc, I wonder whether we borrow the distinction between error and fatal from
    Go's testing package. Such a mechanism would allow the workflow author (and
    indeed the workflow user?) to control when things are error or fatal.
    Somewhat related to this is the question of whether the task author should
    ever make a task fatal, or instead make the errors clear such that the user
    can make the decision to treat an error as fatal (c.f. Go's error handling
    advice)
  * Per point 18 in [Notes on `cue cmd` · cue-lang/cue · Discussion
    #3917](https://github.com/cue-lang/cue/discussions/3917), we need to
    provide some native means for allowing (and indeed preventing) a task to
    support retrying in case of failure. Similarly, we need to natively support
    some notion of timeouts.
* Off the back of looking at Claude and `cue cmd`, we also need the ability to
  control how the process runner fails in some situations. For example, with
  [Hooks reference -
  Anthropic](https://docs.anthropic.com/en/docs/claude-code/hooks) the process
  that handles a hook must exit 2 to block, otherwise it's just advisory. i.e.
  we not only need the ability to control task control flow, but also the
  behaviour of the process running the workflow
* Currently dependencies between tasks are created with a reference to a task or
  a field within a task, e.g. `readFile.contents`. Because tasks can be
  "grouped" within structure, it's natural to want to depend on a group of tasks
  (and possibly with the more formal introduction of task success/failure) the
  success/failure of all contained tasks.
  [cue-lang/cue#2078](https://github.com/cue-lang/cue/issues/2078). Also picked
  up in [cue-lang/cue#1325
  (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-2190876376).
  * [https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue](https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue)
    also provides a good example of how a block/category of tasks might want to
    depend on another task. How can we do that elegantly?
* Very much related to the points about task dependencies (on success/failure)
  but also visualisation, we need to be clear how workflows should be declared
  where one task is somehow conditional on another.
  [cue-lang/cue#1593](https://github.com/cue-lang/cue/issues/1593) and other
  issues highlight that people naturally reach for an `if` comprehension to do
  something conditionally. By contrast, GitHub actions workflows place the `if`
  field within the step. The key difference is the fact that the latter allows
  the potential workflow to be visualised before it is run (if indeed it runs at
  all). "Hiding" tasks behind an `if` comprehension, means that they are not
  actually tasks until "they come into existence". This presents a real UX
  problem, and exacerbates the issue of "I was expecting this command to do X
  but it didn't".
* Building on recent developments in Go's [testing](https://pkg.go.dev/testing)
  package (but also mirroring capabilities in GitHub actions workflow "always
  run this step") we should look to provide some notion of a "cleanup" phase.
  Indeed some tasks, e.g. "make a temporary directory" might by default register
  a cleanup phase task "delete the created temporary task". See
  [cue-lang/cue#1325
  (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-2192457001)
  point 4.
* Introducing the notion of a failure task:
  [cue-lang/cue#3589](https://github.com/cue-lang/cue/issues/3589). This
  allows, under certain conditions, an explicit error. The distinction between
  whether this task would be a fatal or just an error is covered above.
* Caching. [Notes on `cue cmd` · cue-lang/cue · Discussion
  #3917](https://github.com/cue-lang/cue/discussions/3917) (heading "task
  deduplication") picks up the point about task deduplication
  * Specifically the thread that concludes with [this
    comment](https://github.com/cue-lang/cue/discussions/3917#discussioncomment-14256623)
  * In particular it touches on the notion of how caching is, in general, a
    hard/impossible problem to "solve" at the workflow level.
  * General-purpose caching in build systems is often unsafe because it relies
    on an approximation of a process's true inputs, from brittle file timestamps
    in make to more robust but still potentially incomplete content hashing in
    tools like Buildkit. This inherent risk becomes a certainty when remote
    state is involved, as the build system cannot track external network
    dependencies, making the cache non-deterministic and fundamentally
    unreliable.
  * But the thread isn't about caching per se, rather how to avoid unnecessary
    work. An in that respect, it's critical that `cue cmd` v2 makes clear how
    references "work", and where there are bugs today that cause things to be
    brittle (the example in the thread is trivially broken by unifying with `{}`
    instead of a naked reference).
* We should be very clear on, what the pattern is by which people should compose
  commands/tasks.
  [https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue](https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue)
  does a beautiful job of laying this out cleanly. But as [this
  thread](https://github.com/cue-lang/cue/discussions/3917#discussioncomment-14256623)
  highlights, this might have been more by luck than judgement!
  * The pattern should be clear, and it should be easy to get right, intuitive
    etc
  * But the opposite must also be true; it should be hard to do the wrong thing,
    and obvious (error messages, visualised workflow) when such a situation has
    occurred
* Tasks that should only be run "on demand" i.e when referenced.
  [https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue](https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue)
  highlights an interesting mode of `cue cmd` where it runs tasks that are
  referenced from a well-defined task (within the `command` namespace, at a path
  that is made up entirely of regular fields). But where the referenced task
  would otherwise be runnable - e.g. `meta._commands` are not declared with the
  `command` namespace, and they are declared in a hidden field.
  * In this setup, the tasks are run implicitly as part of the command, by
    reference (overlap here, at a meta level, with the evaluator being lazy and
    only evaluating what it needs to).
  * What's critical in this example is that currently the path at which a task
    is "found" is, in essence, the thing that "uniques" it, providing a point of
    synchronisation
  * Where this potentially gets a bit icky is if tasks are actually templates
    (with defaults). Hence we need to have a clear notion of when we are
    establishing a task to be run, or simply creating a template.
* [`run_caddy`](https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue#L243-L248)
  gives an example of a background task. This hints that something like job
  control might be required in the general case.
* Fixing existing bugs with respect to dependency calculation, i.e. dependencies
  between tasks:
  * [cue-lang/cue#1568](https://github.com/cue-lang/cue/issues/1568) and
    friends
  * See also the extensive exchange that leads to the conclusion in
    [https://github.com/cue-lang/cue/discussions/3917#discussioncomment-14256623](https://github.com/cue-lang/cue/discussions/3917#discussioncomment-14256623)

## Controlling and Observing a running workflow

* The ability to control/limit parallelism of tasks. See
  [cue-lang/cue#709](https://github.com/cue-lang/cue/issues/709)
* Issues like [cmd/cue: nested workflow command tasks with non-concrete
  identifiers don't fail #3897](https://github.com/cue-lang/cue/issues/3897)
  are an example of a class of problems where tasks are either not declared
  because the field names are incomplete (in which case they don't get
  "detected") or when tasks don't get run but were expected to, again because of
  a lack of data. The UX issues of [cmd/cue: nested workflow command tasks with
  non-concrete identifiers don't fail
  #3897](https://github.com/cue-lang/cue/issues/3897) were solved by erroring
  in the case that no tasks were found, but this does not solve the general case
  of "we didn't find and run all the tasks we were expecting to". A v2 of cue
  cmd should consider this problem fairly carefully, especially the UX of
  surfacing issues, debugging etc.
  * Related to this is the visualisation/debugging of a workflow. For example,
    one way of extending the extremely primitive current means of generating a
    sequence of mermaid diagrams via `CUE_DEBUG=toolsflow` would be to have `cue
    cmd` v2 spin up a browser, which contains a live-updating mermaid diagram as
    the workflow proceeds. This could further require the user to "hit enter" to
    step forwards with a task, and would give a clear visualisation of "what the
    workflow will do"
  * This point is very much related to people "just wanting to evaluate" the
    workflow itself, to see the form it takes, i.e. what `tools/flow` ultimately
    consumes at the start. People are trying to get more information about what
    is going on, principally at the start but also as a workflow progresses. One
    thing to note here is that if we retain this notion of references between
    tasks creating dependencies, rendering a workflow via `cue eval` or similar
    should absolutely not render the reference task at the point of referencing
    (i.e. where the dependency is created): [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-2192437822).
  * Similarly, people want to run arbitrary commands to inspect the inputs to
    various tasks at any stage during evaluation [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-2192457001).
    Having some form of verbose logging of a workflow (a la GitHub actions logs)
    would go a long way in this respect, but that's incredibly loud/verbose
    when, for example, the "input" to a task is a massive k8s configuration
    object. Such logging would necessarily have to include some sort of line
    prefix which makes clear which tasks are emitting which log lines (because
    tasks execute concurrently).
* The need to "see what a workflow is doing" exists at many levels: visualising
  in a UI (evolving mermaid?), getting line-based output in a terminal,
  visualising in a TUI semi-graphically. We need to think about what kind of
  model of tweaking levels of detail works here.

## Performance

* Serious performance issues arise from situations where, for example, multiple
  files need to be created. Right now, we achieve such a result with a for
  comprehension over a list/map similar. This results in multiple tasks being
  run (concurrently). The problem is that every result results in a full
  re-evaluation of the workflow configuration. Whilst there are likely options
  for improving performance within the evaluator, perhaps we should also
  consider things like:
  * Explicit batch operations. This is kind of how the writefs "hack" we adopted
    for [cuelang.org](http://cuelang.org) configurations work. In that
    situation, we take a JSON configuration that represents the files we want to
    create on disk, and a Go program creates all of those files in a single
    batch. By a similar token, os/file.Create could take a map of file paths to
    contents. Indeed the [cuelang.org](http://cuelang.org) example is even more
    pernicious: there we need to delete a number of files that match some glob
    patterns, and then create a whole load of files. This results is: at least
    one call to expand globs, an `os/file.RemoveAll` call for each file matched
    by the glob, an os/file.Create call for every task we want to create. With a
    full evaluation happening after each task completion, this is extremely
    expensive in the case that the package
  * Implicitly batching operations. For example, it might be possible for the
    workflow scheduler (tools/flow or whatever takes its place) to spot "ah, all
    of these tasks are now ready to run concurrently, I will gather the results
    from all of them together and then perform a single unification with the
    result". Quite how the workflow author would opt in/out of such a setup is
    not clear to me, but in the case of the [cuelang.or](http://cuelang.or)g
    example this would mean just 3 unifications (post glob, delete and then
    create).

## On function calls, capabilities, and more

* A CUE evaluation is hermetic, it cannot have side effects. However we need
  configurations to be written to disk, served via network requests, completed
  with respect to data that exists in a database via a query. That is,
  configuration (the verb, not the noun) is something that involves side
  effects.
  * Tasks like `exec.Run`, `file.Create`, `file.Read`, `http.Get` and more -
    all exist in order to bridge between a CUE value and a side effect (and some
    return value).
  * There is a spectrum that exists between "entirely hermetic evaluation of
    inputs" to "executing can result in any number of side effects".
  * For example, some people have advocated allowing a `cue export` to be able
    to execute code in, for example, a NodeJS function as part of "regular"
    evaluation. Absent careful controls, it would be possible for that function
    to have side effects unintended by the configuration owner, especially if
    you consider that function being provided by a third party.
  * As another example, we don't necessarily want a workflow (which might be
    considered analogous to a bash script) to be able to "do anything it likes"
    - indeed certain parts of a workflow (again especially those where we be
    using a third party command/task) might well need to be carefully
    constrained.
  * Linux has the notion of process capabilities and namespaces that allow
    isolation at the process level of certain kinds of operation. e.g. bind
    mounts in a process filesystem namespace allow a process to operate in a
    controlled way on certain files alone. Network capabilities limit a process'
    ability to do certain things with a network device. These are the primitives
    on top of which Docker is built.
  * A key observation of the previous points is that in the space of CUE, we
    need/want that kind of control at the function/task/workflow level.
  * Indeed this was a key part of CUE's exploration of
    [WASM](https://github.com/cue-lang/cue/discussions/2045), work that is
    currently on hold.
  * All of this is background to highlight that, somewhere in the function
    design and `cue cmd` v2 space, some notion of constraining the capabilities
    of functions/tasks/workflows is needed. Whilst such a solution might be
    implemented at the OS process level (e.g. building atop linux kernel
    capabilities), the evaluator/workflow runner needs to be the enforcer of
    such policies (or in other language the granter of such capabilities).
  * `exec.Run` and platform-specific options. A point nicely picked up in [Notes
    on `cue cmd` · cue-lang/cue · Discussion
    #3917](https://github.com/cue-lang/cue/discussions/3917) (heading "Hermetic
    / Reproducible Environments") is that running a process does not necessarily
    need to be limited to an equivalent of Go's
    [`os/exec.Cmd`](https://pkg.go.dev/os/exec#Cmd) and the output of such a
    command (see also the point above about long-lived tasks, like servers).
    GitHub actions, for example, allows a step to be an action which is itself
    defined in terms of a docker image, the running of the step being an
    instance (container) of that image with appropriate mounts etc. This kind of
    approach is only natively possible on Linux, but is an incredibly powerful
    notion
* To what extent there is overlap between `cue cmd` v2 and any function design.
  Not prompting or proposing anything in particular here, just that we should
  make clear what the anticipated overlap is. In particular the ability, for
  example, in a "regular" CUE evaluation to have non-standard library code
  execute. e.g. a JavaScript function executing to add two numbers.
  * Related is the feedback in [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-3215870661)
    - can we achieve what we want with `cue cmd` v2 via non-hermitic function
    calls? And vice versa: can non-hermetic function calls during evaluation be
    achieved with whatever we propose via `cue cmd` v2? i.e. do they collapse to
    the "same implementation"?
  * One potential answer in this regard is that `if` fields (a la GitHub
    actions) might be required to establish clearly defined and debuggable
    workflows, where the user has much more interest/control over the order of
    execution. This effectively creates a sort of higher order evaluator, built
    on top of the low level one. Whereas a non-hermitic function implementation
    could conceivably be implemented, with certain constraints, as part of the
    existing evaluator.
  * 'this cue export has a non-hermetic hole in it, make sure you use a certain
    flag allowing tool calls'? [cue-lang/cue#1325
    (comment)](https://github.com/cue-lang/cue/issues/1325#issuecomment-3215870661)
* The pattern of dependency injection is not a new concept. For example,
  consider some CUE we depend on that reads some output from a CLI command
  (ignore for one second whether this is a task or not, accepting the fact that
  this would not be possible today because of hermetic evaluation). This CUE
  could directly depend on some standard library primitive for the execution of
  the CLI command. Or else it could rely on something that has the same shape as
  such a standard library primitive, but requiring the actual function
  implementing it to be passed "as an argument". Such a mechanism would allow
  the caller to effectively control the capabilities of any dependency (note
  this is a different use of the word "capabilities" to the precise meaning in
  the Linux kernel). What's not clear is where such a line could/should be
  drawn: a custom `encoding/json.Marshal` function could be passed in?
  * Such a point actually has overlap with the running of a CLI command, making
    a network call etc. Because dependency injection of this sort only solves
    part of the problem
  * Specifically, if we want to limit the filesystem capabilities of a task, in
    the general case this can only be done by the environment in which the
    workflow is being run (e.g. the OS kernel).
* Currently, tasks have to be declared at paths. Those paths don't need to be
  contained by the command being run. Potential entry points within a command
  are discovered via regular fields. Tasks can also be discovered by reference.
  Referenced tasks need not be part of the set of potential entry points. That
  is they can be declared at a path outside of the command's namespace, and even
  in hidden fields.
  * `cue cmd` tasks need to be rooted at a path definitely feels like it has
    strong benefits in terms of debugging, tracing, logging etc.
  * There is a subtlety that (in loose terms) a field is "just" a reference to
    another field, then the target of the reference is the path of the task,
    [for
    example](https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue#L268).
    But as soon as that reference is unified with something (and there may be
    other things that "break" it), the task is rooted at the point of reference.
    Is this too brittle?
  * How does this relate to the question of non-task function calls? Should they
    be required to be declared at a specific path, as opposed to a non-rooted
    expression?

## Other

* We also need a more cross-platform way to read from stdin (or write log
  messages to stderr). This would allow a `cue cmd` workflow to be used as part
  of a Claude Code hooks setup as just one example.
* We should explicitly support `**` as part of globs as a convenience mechanism
  for users. As part of the work on
  [`embed`](https://cuelang.org/docs/howto/embed-files-in-cue-evaluation/) we
  explicitly made the use of `**` an error in order to allow for later allowing
  the use of `**` without silently changing behaviour. `**` is a common pattern,
  and whilst its use can lead to performance situations, this is entirely in the
  control of the caller.
* Rog has done some excellent work in
  [https://review.gerrithub.io/c/cue-lang/cue/+/1171061](https://review.gerrithub.io/c/cue-lang/cue/+/1171061)
  which demonstrate how we can significantly beef up the "templating" of things
  like shell scripts, Go code, HTML etc through what he refers to as tagged
  interpolations. Whilst that is largely orthogonal to workflows, it's clearly a
  very powerful use case: being able to template shell code in a safe way (much
  like Go's [`html/template`](https://pkg.go.dev/html/template)
* As stated elsewhere on many occasions before, `cmd/cue` should not be
  implemented in terms of anything internal, and by extension (over time) `cue
  cmd` v2 (assuming it happens and lives as part of `cmd/cue`) should not rely
  on internals.
* Whether we should implement `@export` or similar
  ([https://github.com/cue-lang/cue/issues/2031](https://github.com/cue-lang/cue/issues/2031))
  for the common use case of "generate these bits of our configuration to these
  files on disk (deleting generated files that match this glob beforehand)".
* In
  [https://github.com/cue-lang/cue/discussions/3917#discussioncomment-14191532](https://github.com/cue-lang/cue/discussions/3917#discussioncomment-14191532)
  David picks up on the point about wanting unit test tasks. "unit test things
  like exit codes, text outputs that I expect/rely on downstream, etc."
* The use of [`||` in
  bash](https://github.com/infogulch/xtemplate/blob/0f1c1fd728e6be99c516e40d09df70fc295b5b67/make_tool.cue#L44)
  is something (along with similar patterns) that we should understand how to
  encode in `cue cmd` v2.

