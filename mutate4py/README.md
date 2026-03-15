# mutate4py

`mutate4py` is the planned Python mutation-testing port of the `mutate4java` model.

## Current status

- repository skeleton created
- package layout initialized
- TDD-oriented test-porting plan documented
- MVP implementation is functional and backed by tests

## Current capabilities

- file-scoped mutation testing
- AST-based mutation discovery
- sidecar manifests for differential reruns
- coverage-based pruning using reused XML or generated `coverage.py` data when available
- baseline-first mutation execution
- worker-copy and multi-worker execution
- verbose progress reporting
- runnable module and installed console-script entrypoints
- Java-inspired CLI and acceptance suite

## Validation

- test suite: `python3 -m unittest discover -s tests -p 'test_*.py'`
- module entrypoint: `PYTHONPATH=src python3 -m mutate4py --help`
- installed console script validated in a temporary virtual environment with `pip install -e .`

## Future improvements

- evaluate `libcst` for higher-fidelity source rewriting
- broaden mutation operators carefully after preserving current parity behavior

## Build artifacts

```bash
python3 -m pip install build
python3 -m build --sdist --wheel
```

For monorepo packaging, use `make package-py` from `/home/cyril/my_projects/mutation_libraries`.

## Planned test categories

- CLI argument parsing
- mutation catalog behavior
- coverage parsing
- subprocess execution
- application orchestration
- end-to-end acceptance behavior
