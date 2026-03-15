# Mutation Libraries Agent Guide

A shared workflow guide for the mutation-library workspace.

This workspace is building language-specific mutation testing libraries inspired by `mutate4java`.

Current implementation order:

1. Go
2. Python
3. Rust

## Project Intent

The goal is to stay as close as practical to `mutate4java` in:

- staged execution flow
- CLI behavior
- mutation site discovery rules
- manifest-driven differential selection
- coverage-guided pruning
- worker-isolated mutation execution
- test-suite shape and acceptance coverage

Primary research and planning docs:

- `README.md`
- `mutate4java-findings.md`
- `mutate4java-deep-dive.md`
- `cross-language-design.md`
- `mvp-roadmap.md`
- `go-mvp-milestones.md`
- `golang-implementation-plan.md`
- `python-mvp-milestones.md`
- `python-implementation-plan.md`

## Core Principles

### 1. Research First, Then Build

Before changing behavior:

1. Identify the matching `mutate4java` behavior and tests.
2. Identify the target language crate/package/module.
3. Identify the smallest failing tests to add first.
4. Check whether the behavior belongs in:
   - CLI
   - analysis
   - coverage
   - manifest
   - execution
   - reporting

Do not invent workflow differences unless the target language truly requires them.

### 2. Keep Ports Behaviorally Aligned

The Java library is the reference behavior.

Ports should preserve, where practical:

- exit-code semantics
- scan behavior
- update-manifest behavior
- baseline-first failure behavior
- timeout behavior
- uncovered-site reporting
- differential selection behavior
- worker-copy behavior

When a port must diverge, document the reason in the repo README or milestone docs.

### 3. Tests First

For every behavior change:

1. Find the corresponding Java test or contract.
2. Port or adapt the test first.
3. Run the test and confirm it fails for the expected reason.
4. Implement the minimum code needed to pass.
5. Run the focused suite.
6. Then run the broader suite.

If the Java contract is not yet portable, capture the closest equivalent test and document the gap.

### 4. Work In Small Verifiable Steps

Each step should be:

- one coherent behavior
- small enough to explain clearly
- independently testable
- easy to revert or refactor

Prefer a series of small green steps over a large rewrite.

### 5. Preserve Determinism

Tests must be deterministic.

- avoid random behavior in tests
- prefer temporary directories and isolated fixtures
- keep source mutation offsets and scope ids stable under test
- do not let parallel execution make assertions order-dependent

### 6. Quality Gates

For active ports:

1. Run the focused tests for the changed slice.
2. Run the repo-wide test command.
3. Keep behavior and output deterministic.
4. Update docs when parity or roadmap assumptions change.

Do not start the next behavior slice on top of failing tests.

## Workflow Patterns

### New Capability

1. Map to the closest Java behavior.
2. Add/port failing tests first.
3. Implement the minimal code.
4. Run targeted tests.
5. Run the full repo test suite.
6. Update docs if the architecture or parity story changed.

### Bug Fix

1. Reproduce with a failing test.
2. Fix with the smallest coherent change.
3. Verify related behavior still passes.
4. Document any parity implications.

### Refactoring

1. Confirm behavior with tests first.
2. Refactor incrementally.
3. Run tests after each meaningful step.
4. Stop if behavior or parity becomes unclear.

## Repository Layout

### Shared Workspace

- `README.md` - workspace index and status
- `session-log.md` - chronological work log
- `AGENTS.md` - this workflow guide

### Go Port

- `mutate4go/`
- primary command: `go test ./...`

### Python Port

- `mutate4py/`
- primary command: `python3 -m unittest discover -s tests -p 'test_*.py'`

### Rust Port

- not started yet
- follow the same staged approach once created

## Development Commands

### Go

```bash
go test ./...
go run ./cmd/mutate4go --help
```

### Python

```bash
python3 -m unittest discover -s tests -p 'test_*.py'
```

## Documentation Rules

- Record meaningful progress in `session-log.md`.
- Update milestone docs when a milestone meaningfully advances.
- Update `README.md` when workspace status changes.
- Keep the parity story explicit: note what matches Java and what still differs.

## Git Rules

- Do not commit unless explicitly asked.
- Do not commit generated artifacts or temporary coverage files.
- Keep worktrees clean of accidental test fixtures that are not part of the repo.

## Packaging Rules

- Treat each library as independently packageable and releasable.
- Do not package or publish a new version if there are no code or behavior changes for that library.
- Keep packaging metadata local to each implementation repo:
  - `mutate4go/`
  - `mutate4py/`
  - `mutate4rs/`
- When adding release automation later, preserve per-library versioning rather than a single workspace version.

## Anti-Patterns

- Implementing behavior before writing the failing test
- Adding broad new features without matching them to Java behavior
- Letting output order become nondeterministic
- Diverging across languages without documenting why
- Expanding mutation operators before core orchestration is stable

## Quality Checklist

Before considering a slice done:

- [ ] Matching Java behavior was identified
- [ ] Failing tests were added first
- [ ] Focused tests pass
- [ ] Full repo tests pass
- [ ] Docs were updated if needed
- [ ] Known parity gaps remain documented

## Final Rule

If a change does not improve parity, clarity, or test confidence, it probably does not belong in the current slice.
