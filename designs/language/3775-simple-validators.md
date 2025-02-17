
# Remove some Validator Rewriting

## Objective

To improve the predictability and consistency of CUE’s validator behavior by disallowing the unification of two non-concrete values into a concrete value. This eliminates inconsistencies in bound simplifications and simplifies reasoning about constraint reductions.

## Background

CUE currently allows reductions of some constraints into concrete values. This behavior, while convenient in some cases, introduces inconsistencies and unpredictability in constraint resolution.

For example:

```cue
int & >=1 && <2
```

can be reduced to:

```cue
1
```

Even though this is a straightforward case, it may still be surprising to users.

### Example of problematic case

There are also less straightforward cases.

```cue
a: *1 | int
a: <max
max: int
```

Results in:

```cue
min: (*1 | int) & <max
max: int
```

Since `max` is not a concrete value, `<max` remains unresolved, preventing a concrete resolution of `min`.

The following case, however, demonstrates some possibly undesirable simplifications:
```cue
min: *1 | int
min: <max
max: >min
```

In this case, it simplifies perhaps unexpectedly to
```cue
min: 1   // default taken
max: >1
```

CUE simplifies bounds in some cases. For instance, it rewrites `<= (<3)` to `<3`
to `<3` resulting in a concrete bound, even though the argument to the bound
was not concrete.

In this case, CUE simplifies `< (>min)` to `number`, which unifies with both disjuncts, giving the perhaps somewhat surprising result.

So here we saw that resolutions can differ more than expected based son some small change, where the result depends on whether bound simplification is implemented and how accurate this implementation is.

### Interaction with other (upcoming) proposals

We intend to make the use of `==` legal for comparing CUE values of different types. This opens up the possibility of having structured map keys, a principled approach to associative lists, as well as allowing `==` as a validator for concrete values.

The use of `==` as a unary operator has many applications. However, it is also an obvious candidate for the kind of simplification we are proposing to eliminate. For instance, `== 3` could trivially be simplified to `3`.

This proposal removes the inconsistencies that would result from such applying implicit simplifications in some cases, but not in others.


## Proposal

* Disallow unification of two non-concrete values into a concrete value.  For instance: `>=1 & <2 & int` should not simplify to `1`.

* Disallow the simplification of non-concrete validators. For instance, `<=(<3)` should not simplify to `<3`, but rather be an (incomplete) error.

For clarity, we will keep allowing simplifications of the unification of two concrete validators, like `<3 & <5` to `<3`.


## Design Details

The implementation of this proposal will mostly consist of deleting or disabling swats of code in the internal packages `adt` and `export`.

Note that we can still safely keep the simplification of the unification of two concrete validators to non-concrete values, like `<3 & <5` to `<3`, as this will not alter semantics in any way by removing relationships.


### Backward Compatibility Concerns

The resulting change may lead to CUE configurations to not resolve where they previously would. In some cases this may expose bugs in the configuration, in other a desired simplification effect may be lost.

An initial investigation on Unity show zero changes as a result of this proposal. We could still provide a backwards compatibility mode, but would disable this by default immediately on first rollout.

### Performance Considerations

Not having to simplify bounds might lead to slightly improved performance. However, there is a possibility that the lack of resulting elimination might expand the search space. Either way, we expect the impact to be minimal.


## Alternatives Considered

### 1. Implementing Comprehensive Bound Simplifications
- Would require full coverage of all possible simplifications.
- Extremely complex and still may not guarantee consistency.

### 2. Leaving Current Behavior Unchanged
- Continues existing inconsistencies.
- Forces users to manually work around unpredictable behavior.

The proposed approach (removing non-concrete simplifications) offers the best balance between predictability, maintainability, and usability.


