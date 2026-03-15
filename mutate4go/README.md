# mutate4go

`mutate4go` is a Go mutation-testing prototype derived from the `mutate4java` architecture.

## Current capabilities

- mutate one `.go` file at a time
- AST-based mutation discovery
- scan mode
- update-manifest mode
- sidecar manifest storage and changed-scope selection
- line-coverage pruning from Go coverprofiles
- baseline-first execution
- timeout-based mutant killing
- sequential execution and copied-worker parallel execution
- Java-inspired CLI and acceptance tests

## Current mutation set

- `true <-> false`
- `0 <-> 1`
- `== != < <= > >=`
- `&& <-> ||`
- `+ <-> -`, `* <-> /`
- unary `!x -> x`
- unary `-x -> x`

## Known gaps vs mutate4java

- sidecar manifests are used instead of embedded source-file manifests
- no `nil` replacement mutants yet
- scope tracking currently centers on file and function/method scopes
- test suite mirrors the Java suite categories and behavior, but not every Java case has been ported yet

## Run tests

```bash
go test ./...
```

## Run the CLI

```bash
go run ./cmd/mutate4go --help
```

## Build artifact

```bash
go build -o dist/mutate4go ./cmd/mutate4go
```

For monorepo packaging, use `make package-go` from `/home/cyril/my_projects/mutation_libraries`.
