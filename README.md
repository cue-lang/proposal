# Proposing changes to CUE
CUE Project Design Documents

## Introduction

Significant changes to the language, libraries, or tools
(which includes API changes in the main repo and all cue-lang.org repos
as well as command-line changes to the `cue` command)
must be first discussed, and sometimes formally documented,
before they can be implemented.

This document describes the process for proposing, documenting, and
implementing changes to the CUE project.

We are hereby following a similar approach that used by Go, and also other
languages. This process is new for CUE and will likely be adjusted over
time.

## The Proposal Process

The proposal process is the process for reviewing a proposal and reaching
a decision about whether to accept or decline the proposal.

1. The proposal author [creates a brief discussion](https://github.com/cue-lang/cue/discussions/new?category=proposal) describing the proposal.\
   Note: There is no need for a design document at this point.\

2. A discussion on the GitHub Discussions aims to triage the proposal into one of three outcomes:
     - accept proposal, or
     - decline proposal, or
     - ask for a design doc.

   If the proposal is accepted or declined, the process is done.
   Otherwise the discussion is expected to identify concerns that
   should be addressed in a more detailed design.

3. The proposal author writes a [design doc](#design-documents) to work out details of the proposed
   design and address the concerns raised in the initial discussion.

4. Once comments and revisions on the design doc wind down, there is a final
   discussion on the issue, to reach one of two outcomes:
    - Accept proposal or
    - decline proposal.

After the proposal is accepted or declined (whether after step 2 or step 4),
implementation work proceeds in the same way as any other contribution.

## Detail

### Goals

- Make sure that proposals get a proper, fair, timely, recorded evaluation with
  a clear answer.
- Make past proposals easy to find, to avoid duplicated effort.
- If a design doc is needed, make sure contributors know how to write a good one.

### Definitions

- A **proposal** is a suggestion filed as a GitHub discussion.
- A **design doc** is the expanded form of a proposal, written when the
  proposal needs more careful explanation and consideration.

### Scope

The proposal process should be used for any notable change or addition to the
language, libraries and tools.
“Notable” includes API changes in the main repo and all golang.org/x repos,
as well as command-line changes to the `go` command.
It also includes visible behavior changes in existing functionality.
Since proposals begin (and will often end) with the filing of an issue, even
small changes can go through the proposal process if appropriate.
Deciding what is appropriate is matter of judgment we will refine through
experience.
If in doubt, file a proposal.

There is a short list of changes that are typically not in scope for the proposal process:

- Making API changes in internal packages, since those APIs are not publicly visible.
- Feature requests that do not propose a particular design.

Again, if in doubt, file a proposal.

### Compatibility

CUE has not reached v1 yet, and does not have a compatibility guarantee.
That said, we aim to make as few breaking changes as possible.
Changes to the language should not considered lightly and only be
suggested if they are thought to have significant benefit.
For backwards incompatible changes there should be a clear migration
path and as well as tooling design that can help in automated rewrites.

### Language changes

Language changes require an additional level of prudence.
Any change to the CUE language should

- address an important issue for many people,
- have minimal impact on everybody else, and
- come with a clear and well-understood solution.


### Design Documents

As noted above, some (but not all) proposals need to be elaborated in a design document.

- The design doc should be checked in to [the proposal repository](https://github.com/cue-lang/proposal/) as `designs/NNNN-shortname.md`,
where `NNNN` is the GitHub discussion number and `shortname` is a short name
(a few dash-separated words at most).

- The design doc should follow [the template](design/TEMPLATE.md). [TODO]

- The design doc should address any specific concerns raised during the initial discussion.

- It is expected that the design doc may go through multiple checked-in revisions.
New design doc authors may be paired with a design doc "shepherd" to help work on the doc.

- Design documents should be wrapped around the 80 column mark.
[Each sentence should start on a new line](http://rhodesmill.org/brandon/2012/one-sentence-per-line/)
so that comments can be made accurately and the diff kept shorter.
  - In Emacs, loading `fill.el` from this directory will make `fill-paragraph` format text this way.

- Comments on PRs should be restricted to grammar, spelling,
or procedural errors related to the preparation of the proposal itself.
All other comments should be addressed to the related GitHub discussion.


### Quick Start for Experienced Committers

Experienced committers who are certain that a design doc will be
required for a particular proposal
can skip steps 1 and 2 and include the design doc with the initial issue.

In the worst case, skipping these steps only leads to an unnecessary design doc.

### Proposal Review

Design docs are currently reviewed periodically without a
predetermined cadence.

The principal goal of the review meeting is to make sure that proposals
are receiving attention from the right people,
involving relevant developers, raising important questions,
pinging lapsed discussions, and generally trying to guide discussion
toward agreement about the outcome.
The discussion itself is expected to happen on GitHub Discussions,
so that anyone can take part.

The proposal review meetings also identify issues where
consensus has been reached and the process can be
advanced to the next step (by marking the proposal accepted
or declined or by asking for a design doc).

The proposal review group can, at their discretion, make exceptions for
proposals that need not go through all the stages, fast-tracking them
to Likely Accept/Likely Decline or even Accept/Decline, such as for
proposals that do not merit the full review or that need to be considered
quickly due to pending releases.

A more precise process is yet to be defined and will be posted here
once it is established.
