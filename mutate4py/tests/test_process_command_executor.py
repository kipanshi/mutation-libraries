from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.exec import ProcessCommandExecutor


class ProcessCommandExecutorTest(TestCase):
    def test_captures_successful_command_output(self) -> None:
        result = ProcessCommandExecutor().run(
            ["sh", "-c", "printf ok"],
            self._temp_dir(),
        )

        self.assertEqual(0, result.exit_code)
        self.assertEqual("ok", result.output)
        self.assertFalse(result.timed_out)

    def test_returns_timeout_exit_code_when_command_takes_too_long(self) -> None:
        result = ProcessCommandExecutor().run(
            ["sh", "-c", "sleep 1"],
            self._temp_dir(),
            timeout_millis=10,
        )

        self.assertEqual(124, result.exit_code)
        self.assertTrue(result.timed_out)

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()
