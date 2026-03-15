from pathlib import Path
import subprocess
from unittest import TestCase


class MainModuleTest(TestCase):
    def test_python_module_help_exit_code(self) -> None:
        root = Path(__file__).resolve().parents[1]
        completed = subprocess.run(
            ["python3", "-m", "mutate4py", "--help"],
            cwd=root,
            env={"PYTHONPATH": str(root / "src")},
            capture_output=True,
            text=True,
            check=False,
        )

        self.assertEqual(0, completed.returncode)
        self.assertIn("Usage: mutate4py <file.py> [options]", completed.stdout)
