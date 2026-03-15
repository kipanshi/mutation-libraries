# mutation-libraries

Monorepo for three mutation-testing ports inspired by `unclebob/mutate4java`.

Current docs and commands are written for Linux, tested on this local environment.

## Packages

- `mutate4go` - Go port
- `mutate4py` - Python port
- `mutate4rs` - Rust port

All three aim to preserve the same behavioral intent:

- analyze one source file at a time
- discover mutation sites from parsed syntax
- store scope hashes in sidecar manifests
- support differential reruns with `--since-last-run`
- support explicit full reruns with `--mutate-all`
- prune by line coverage when available
- require a green baseline before mutant runs
- run mutants in isolated worker copies
- emit deterministic text summaries

## Requirements

Base tools:

- `git`
- `make`

Language toolchains used in this workspace:

- Go `1.18+`
- Python `3.10+`
- Rust stable

Packaging helpers:

- Python packaging: `python3 -m pip install build`

## Workspace Docs

- `AGENTS.md` - workspace process and packaging rules
- `cross-language-design.md` - shared architecture direction
- `mvp-roadmap.md` - implementation order and goals
- `go-mvp-milestones.md` - Go milestones
- `python-mvp-milestones.md` - Python milestones
- `rust-mvp-milestones.md` - Rust milestones
- `PUBLISH.md` - release and publishing instructions
- `session-log.md` - execution log

## Quick Start

### Go

```bash
cd mutate4go
go test ./...
go run ./cmd/mutate4go --help
```

### Python

```bash
cd mutate4py
python3 -m unittest discover -s tests -p 'test_*.py'
PYTHONPATH=src python3 -m mutate4py --help
```

### Rust

```bash
cd mutate4rs
cargo test
cargo run -- --help
```

## Release Workflow

From the monorepo root:

```bash
make test
make self-check
make package
make release
make release-check
```

Artifacts are written to `dist/`:

- `dist/mutate4go/` - local Go CLI binary
- `dist/mutate4py/` - Python sdist and wheel
- `dist/mutate4rs/` - Rust `.crate` package
- `dist/self-check/` - self-hosting mutation outputs for all three ports

Package a single library:

```bash
make package-go
make package-py
make package-rs
```

Package only changed libraries when running inside a git repo:

```bash
make release-go
make release-py
make release-rs
```

If no git repo is detected, release packaging falls back to packaging all libraries.

## Separate Packaging Rule

Each library is independently packageable and releasable.

- do not cut a new package version for a library with no code or behavior changes
- version each library independently when release automation is added

## Manifest Files

The sidecar manifests under `.mutate/manifests/` are intended to be kept with the project when you want `--since-last-run` to work across machines and commits.

- commit `.mutate/manifests/`
- do not commit transient `.mutate/coverage/` or `.mutate/workers/`

## End-to-End Self-Checks

The libraries have been exercised on their own codebases:

- `mutate4go` on `cli.go`
- `mutate4py` on `src/mutate4py/cli.py`
- `mutate4rs` on `src/cli.rs`

Each selected self-hosting mutant was killed successfully.
