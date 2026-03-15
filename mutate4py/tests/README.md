# mutate4py Test Plan

The Python port should mirror the shape of the Java and Go suites as closely as practical.

## Planned modules

- `test_cli_arguments_parser.py`
- `test_mutation_catalog.py`
- `test_coverage_parser.py`
- `test_process_command_executor.py`
- `test_process_test_command_executor.py`
- `test_cli_application.py`
- `test_main_acceptance.py`

## TDD approach

1. port the behavioral contract from Java first
2. write failing tests for each slice
3. implement the smallest working feature set
4. keep the Python suite aligned with Go and Java as features expand
