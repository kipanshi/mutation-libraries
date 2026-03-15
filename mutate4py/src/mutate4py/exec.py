from __future__ import annotations

from dataclasses import dataclass
import subprocess
import sys
import time


@dataclass(frozen=True)
class CommandResult:
    exit_code: int
    output: str
    duration_millis: int
    timed_out: bool


@dataclass(frozen=True)
class TestRun:
    exit_code: int
    output: str
    duration_millis: int
    timed_out: bool


class ProcessCommandExecutor:
    def run(
        self, command: list[str], working_directory: str, timeout_millis: int = 0
    ) -> CommandResult:
        started = time.monotonic()
        try:
            completed = subprocess.run(
                command,
                cwd=working_directory,
                capture_output=True,
                text=True,
                timeout=timeout_millis / 1000 if timeout_millis > 0 else None,
                check=False,
            )
            output = (completed.stdout or "") + (completed.stderr or "")
            return CommandResult(
                exit_code=completed.returncode,
                output=output,
                duration_millis=int((time.monotonic() - started) * 1000),
                timed_out=False,
            )
        except subprocess.TimeoutExpired as exc:
            output = _coerce_output(exc.stdout) + _coerce_output(exc.stderr)
            return CommandResult(
                exit_code=124,
                output=output,
                duration_millis=int((time.monotonic() - started) * 1000),
                timed_out=True,
            )


class ProcessTestCommandExecutor:
    def __init__(self, command: list[str] | None = None) -> None:
        self._command = command or [
            sys.executable,
            "-m",
            "unittest",
            "discover",
            "-s",
            "tests",
            "-p",
            "test_*.py",
        ]
        self._shell_command: str | None = None
        self._executor = ProcessCommandExecutor()

    def with_command(self, command: str) -> "ProcessTestCommandExecutor":
        updated = ProcessTestCommandExecutor(self._command)
        updated._shell_command = command
        return updated

    def run_tests(self, project_root: str, timeout_millis: int) -> TestRun:
        command = self._command
        if self._shell_command is not None:
            command = ["sh", "-lc", self._shell_command]
        result = self._executor.run(command, project_root, timeout_millis)
        return TestRun(
            exit_code=result.exit_code,
            output=result.output,
            duration_millis=result.duration_millis,
            timed_out=result.timed_out,
        )


def _coerce_output(value: str | bytes | None) -> str:
    if value is None:
        return ""
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return value
