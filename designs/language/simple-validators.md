
title: Validator Simplication

Proposal

we propose that the unification of two non-concrete values cannot result
in a concrete value.


Background

Currently, CUE allows reductions of some constraints into concrete values.
For instance:
`int & >=1 && <2` can be reduced to `1`.

This seems convenient, but it can lead to inconsistent results:
- Users may not want this side effect.
- Users might expect concrete results, where simplifications are not possible
  or simply ot implemented.

There can also be more insidious cases. For instance,
```cue
a: *1 | int
a: <max
max: int
```
results in
```cue
min: (*1 | int) & <max
max: int
```
As `max` is not concrete, it cannot resolve `<max`,
leaving the expression unresolved.

Now consder this example:
```
min: *1 | int
min: <max
max: >min
```
Here some of the bound simplifications kick into affect. For instance, CUE
simplifies `<=(<3)` to `<3`. In this case, it simplifies `<(>min)` to `number`.
This then unifies with both disjuncts, resulting in:
```
min: 1   // default taken
max: >1
```
Note that due to a bug in V2, this ends up partially evaluated which then
results in an arguably correct cyclic error. But without this bug, it
would result in the same.

In summary:
- It is hard to get bound reduction correct
- As it is stupendous amount of work to cover all bound simplifications,
  the result will alwaysy be inconsistent.
- It is hard to predict when simplifications will kick in, and when not.

The simplest solution is to not do such simplifications at all.

NOte that we will still allow simplifications of the unification of
"concrete bounds", such as `<3 & <5` to `<3`.



