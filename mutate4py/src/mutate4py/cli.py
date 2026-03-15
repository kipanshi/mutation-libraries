from __future__ import annotations

import os

from mutate4py.model import CliArguments, CliMode


def parse_args(args: list[str]) -> CliArguments:
    if args == ["--help"]:
        return CliArguments(mode=CliMode.HELP, file_args=[])

    file_args: list[str] = []
    lines: set[int] = set()
    scan = False
    update_manifest = False
    reuse_coverage = False
    since_last_run = False
    mutate_all = False
    timeout_factor = 10
    mutation_warning = 50
    max_workers = max(1, (os.cpu_count() or 1) // 2)
    test_command: str | None = None
    verbose = False

    index = 0
    while index < len(args):
        arg = args[index]
        if not arg.startswith("--"):
            file_args.append(arg)
            index += 1
            continue

        if arg == "--scan":
            scan = True
        elif arg == "--update-manifest":
            update_manifest = True
        elif arg == "--reuse-coverage":
            reuse_coverage = True
        elif arg == "--since-last-run":
            since_last_run = True
        elif arg == "--mutate-all":
            mutate_all = True
        elif arg == "--verbose":
            verbose = True
        elif arg == "--lines":
            index += 1
            if index >= len(args):
                raise ValueError("--lines requires a value")
            lines = _parse_lines(args[index])
        elif arg == "--timeout-factor":
            timeout_factor, index = _parse_positive_int(args, index, "--timeout-factor")
        elif arg == "--mutation-warning":
            mutation_warning, index = _parse_positive_int(
                args, index, "--mutation-warning"
            )
        elif arg == "--max-workers":
            max_workers, index = _parse_positive_int(args, index, "--max-workers")
        elif arg == "--test-command":
            index += 1
            if index >= len(args):
                raise ValueError("--test-command requires a value")
            test_command = args[index].strip()
            if not test_command:
                raise ValueError("--test-command must not be blank")
        else:
            raise ValueError(f"Unknown option: {arg}")
        index += 1

    if len(file_args) == 0:
        raise ValueError("mutate4py requires exactly one Python file")
    if len(file_args) != 1:
        raise ValueError("mutate4py accepts exactly one Python file")
    if not file_args[0].endswith(".py"):
        raise ValueError("mutate4py target must be a .py file")
    if lines and since_last_run:
        raise ValueError("--lines may not be combined with --since-last-run")

    return CliArguments(
        mode=CliMode.EXPLICIT_FILES,
        file_args=file_args,
        lines=lines,
        scan=scan,
        update_manifest=update_manifest,
        reuse_coverage=reuse_coverage,
        since_last_run=since_last_run,
        mutate_all=mutate_all,
        timeout_factor=timeout_factor,
        mutation_warning=mutation_warning,
        max_workers=max_workers,
        test_command=test_command,
        verbose=verbose,
    )


def _parse_positive_int(args: list[str], index: int, name: str) -> tuple[int, int]:
    index += 1
    if index >= len(args):
        raise ValueError(f"{name} requires a value")
    try:
        value = int(args[index])
    except ValueError as exc:
        raise ValueError(f"{name} must be a positive integer") from exc
    if value <= 0:
        raise ValueError(f"{name} must be a positive integer")
    return value, index


def _parse_lines(value: str) -> set[int]:
    stripped = value.strip()
    if not stripped or stripped.strip(",") == "":
        raise ValueError("--lines requires at least one line number")
    result: set[int] = set()
    for part in stripped.split(","):
        part = part.strip()
        if not part:
            continue
        try:
            line = int(part)
        except ValueError as exc:
            raise ValueError("--lines must be a positive integer") from exc
        if line <= 0:
            raise ValueError("--lines must be a positive integer")
        result.add(line)
    if not result:
        raise ValueError("--lines requires at least one line number")
    return result
