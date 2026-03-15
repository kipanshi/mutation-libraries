# Mutation Testing Wiki

Mutation testing checks whether your tests can detect small, realistic code changes.

## How It Works

- start with correct code
- make a tiny change, called a mutant
- run the tests
- if tests fail, the mutant is killed
- if tests still pass, the mutant survived

## Example

Original:

```python
if total > limit:
```

Mutant:

```python
if total >= limit:
```

If your tests still pass, they may not really verify the boundary behavior.

## Why It Is Useful

- finds weak assertions
- finds missing edge-case tests
- shows where code is executed but not meaningfully tested
- gives better confidence than line coverage alone
- helps prevent tests that run code but do not check enough

## Why Coverage Is Not Enough

- line coverage tells you code was executed
- mutation testing tells you whether wrong behavior would be caught

So:

- high coverage + poor mutation score = shallow tests
- moderate coverage + strong mutation score = often better tests

## What Kinds Of Bugs It Helps Expose

- off-by-one boundaries
- flipped booleans
- wrong comparisons
- missing negations
- weak default handling
- logic branches that are never truly asserted

## Main Outputs

- killed mutants: tests are strong there
- survived mutants: tests are weak or missing
- uncovered mutants: code not exercised
- timed out mutants: possible infinite loop or performance sensitivity

## Tradeoffs

- much slower than normal tests
- some mutants are equivalent, meaning behavior did not really change
- needs good tooling to stay usable
- best used selectively or incrementally, not always on the whole codebase

## Best Practical Use

- on changed files in CI
- on critical logic modules
- before important releases
- during test-hardening work

In this project’s case, the useful principle is:

- mutate one file
- store scope hashes
- rerun only changed scopes
- use coverage to prune useless work

That makes mutation testing far more practical for real teams.

## 1. How To Interpret Mutation Score

Mutation score is usually:

```text
killed mutants / total relevant mutants
```

Often uncovered or invalid mutants are excluded from the denominator, depending on the tool.

Example:

- 80 killed
- 10 survived
- 10 uncovered

Possible reported score:

```text
80 / 90 = 88.9%
```

How to read it:

- `90%+` usually means tests are very strong in that area
- `70% - 90%` is often decent but worth reviewing survivors
- below `70%` often means important gaps or weak assertions

What matters more than the raw number:

- where survivors cluster
- whether critical logic has survivors
- whether score improves over time on changed code

Good practice:

- use score as a diagnostic, not a vanity metric
- inspect surviving mutants, not just the percentage
- track score per file or module, especially for critical code

## 2. What Equivalent Mutants Are

An equivalent mutant is a code change that looks different but behaves the same.

Example idea:

- changing an expression in a way that does not alter observable behavior
- mutating dead code that can never affect output
- replacing logic with something compiler/runtime semantics treat as equivalent in context

Why they matter:

- tests cannot kill them because there is no behavioral difference to detect
- they can unfairly lower mutation score
- they are one reason mutation testing can never be fully automatic

Typical sources of equivalent mutants:

- algebraic identities
- unreachable branches
- redundant boolean logic
- language-specific semantics that make two expressions effectively identical

How teams handle them:

- keep the mutation operator set conservative
- suppress noisy mutant types if they are frequently equivalent
- accept that some survivors are not actionable
- review survivors by importance, not blindly

## 3. How To Introduce Mutation Testing Without Slowing Everyone Down

The wrong way:

- run full mutation testing on the whole repo on every commit

That is usually too slow and too noisy.

Better rollout:

### Step 1: Start Small

- run it only on critical business logic
- pick a few modules where correctness matters most
- use it as a test-quality tool, not a hard gate yet

### Step 2: Use Changed-File Or Changed-Scope Runs

- mutate only files touched in the branch
- even better, mutate only changed scopes inside those files
- this is exactly why sidecar manifests and scope hashing are valuable

### Step 3: Use Coverage Pruning

- skip obviously uncovered mutants
- this saves time and keeps reports focused

### Step 4: Review Survivors In PRs

- ask whether a surviving mutant reveals a missing test
- add tests for important survivors
- ignore or document equivalent/noisy ones

### Step 5: Add Soft Gates Before Hard Gates

Examples:

- warn on new survivors in changed files
- require mutation checks only for critical modules
- gate on “no important new survivors” rather than a global percentage

### Step 6: Keep It Developer-Friendly

- provide a fast local command
- provide clear output with file, line, and mutation description
- store manifests so reruns are cheap
- use worker copies and parallelism carefully

Good long-term team workflow:

- developers run targeted mutation tests locally on changed logic
- CI runs mutation testing on changed files or critical directories
- release checks run broader mutation sweeps when needed

That gives most of the value of mutation testing without turning it into a constant drag on development speed.
