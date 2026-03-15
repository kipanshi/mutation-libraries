from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.exec import ProcessTestCommandExecutor


class ProcessTestCommandExecutorTest(TestCase):
    def test_captures_successful_test_run_output(self) -> None:
        result = ProcessTestCommandExecutor(["sh", "-c", "printf ok"]).run_tests(
            self._temp_dir(), 0
        )

        self.assertEqual(0, result.exit_code)
        self.assertEqual("ok", result.output)
        self.assertFalse(result.timed_out)

    def test_returns_timeout_exit_code_when_test_run_takes_too_long(self) -> None:
        result = ProcessTestCommandExecutor(["sh", "-c", "sleep 1"]).run_tests(
            self._temp_dir(), 10
        )

        self.assertEqual(124, result.exit_code)
        self.assertTrue(result.timed_out)

    def test_starts_shell_command_override_in_target_directory(self) -> None:
        result = (
            ProcessTestCommandExecutor()
            .with_command("printf ok")
            .run_tests(self._temp_dir(), 0)
        )

        self.assertEqual(0, result.exit_code)
        self.assertEqual("ok", result.output)
        self.assertFalse(result.timed_out)

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()
