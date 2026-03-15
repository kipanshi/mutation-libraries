# mutate4rs

`mutate4rs` is the Rust mutation-testing prototype inspired by `mutate4java`.

## Current status

- Rust MVP Milestone 1 is green.
- Rust subprocess execution and initial application help/usage slices are green.

## Current capabilities

- file-scoped Rust target parsing
- CLI argument parsing with Java/Go/Python-aligned option behavior
- AST-based mutation discovery using `syn`
- function-scope hashing and module hashing
- line-based coverage parsing from LCOV data
- subprocess command execution with timeout handling
- test-command execution with shell override support
- manifest storage and changed-scope detection
- scan mode and update-manifest mode
- baseline failure handling, mutation execution, survivor exit behavior, and uncovered-site reporting
- `--since-last-run` and `--mutate-all` differential-selection behavior
- Java/Go/Python-style summary diagnostics for changed and uncovered mutation surface
- worker-copy execution and `--max-workers` support
- runnable `mutate4rs` binary entrypoint
- verbose progress reporting with `--verbose`
- reuse-coverage warning behavior
- real temporary Cargo project acceptance coverage

## Run tests

```bash
cargo test
cargo run -- --help
```

## Build artifacts

```bash
cargo build --release
cargo package --allow-dirty
```

For monorepo packaging, use `make package-rs` from `/home/cyril/my_projects/mutation_libraries`.

## Packaging note

- `mutate4rs` is packaged independently from `mutate4go` and `mutate4py`.
- Do not cut a new package version when this crate has no code or behavior changes.
