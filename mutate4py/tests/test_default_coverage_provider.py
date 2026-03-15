from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.app import DefaultCoverageProvider
from mutate4py.exec import CommandResult, TestRun


class DefaultCoverageProviderTest(TestCase):
    def test_generates_coverage_and_parses_xml_when_available(self) -> None:
        root = self._temp_dir()
        baseline_executor = StubBaselineExecutor(TestRun(0, "baseline ok", 10, False))
        command_executor = StubCommandExecutor(
            CommandResult(0, "coverage ok", 12, False),
            coverage_xml=(
                '<coverage><packages><package name="demo"><classes>'
                '<class name="demo.sample" filename="demo/sample.py">'
                '<lines><line number="2" hits="1"/></lines>'
                "</class></classes></package></packages></coverage>"
            ),
        )
        provider = DefaultCoverageProvider(
            baseline_executor,
            command_executor=command_executor,
            coverage_available=True,
        )

        run = provider.generate_coverage(root, reuse=False)

        self.assertEqual(0, run.baseline.exit_code)
        self.assertTrue(run.report_available)
        self.assertTrue(run.report.covers("demo/sample.py", 2))
        self.assertEqual(1, baseline_executor.invocations)
        self.assertEqual(2, len(command_executor.commands))

    def test_falls_back_when_coverage_is_unavailable(self) -> None:
        root = self._temp_dir()
        baseline_executor = StubBaselineExecutor(TestRun(0, "baseline ok", 10, False))
        command_executor = StubCommandExecutor(CommandResult(0, "unused", 12, False))
        provider = DefaultCoverageProvider(
            baseline_executor,
            command_executor=command_executor,
            coverage_available=False,
        )

        run = provider.generate_coverage(root, reuse=False)

        self.assertEqual(0, run.baseline.exit_code)
        self.assertFalse(run.report_available)
        self.assertEqual(0, len(command_executor.commands))

    def test_skips_generation_when_custom_test_command_mode_disables_it(self) -> None:
        root = self._temp_dir()
        baseline_executor = StubBaselineExecutor(TestRun(0, "baseline ok", 10, False))
        command_executor = StubCommandExecutor(CommandResult(0, "unused", 12, False))
        provider = DefaultCoverageProvider(
            baseline_executor,
            command_executor=command_executor,
            coverage_available=True,
            allow_generation=False,
        )

        run = provider.generate_coverage(root, reuse=False)

        self.assertEqual(0, run.baseline.exit_code)
        self.assertFalse(run.report_available)
        self.assertEqual(0, len(command_executor.commands))

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()


class StubBaselineExecutor:
    def __init__(self, baseline: TestRun):
        self.baseline = baseline
        self.invocations = 0

    def run_tests(self, project_root: str, timeout_millis: int) -> TestRun:
        self.invocations += 1
        return self.baseline


class StubCommandExecutor:
    def __init__(self, result: CommandResult, coverage_xml: str | None = None):
        self.result = result
        self.coverage_xml = coverage_xml
        self.commands: list[list[str]] = []

    def run(
        self, command: list[str], working_directory: str, timeout_millis: int = 0
    ) -> CommandResult:
        self.commands.append(command)
        if self.coverage_xml is not None:
            path = Path(working_directory, ".mutate", "coverage", "coverage.xml")
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(self.coverage_xml, encoding="utf-8")
        return self.result
