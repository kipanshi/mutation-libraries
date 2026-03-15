from io import StringIO
from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.app import Application


class MainAcceptanceTest(TestCase):
    def test_mutates_a_real_python_project_with_custom_test_command(self) -> None:
        root = self._temp_dir()
        self._write_passing_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/flag.py",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(0, exit_code)
        self.assertIn("KILLED demo/flag.py:2 replace True with False", out.getvalue())
        self.assertIn("Summary: 1 killed, 0 survived, 1 total.", out.getvalue())
        self.assertEqual("", err.getvalue())

    def test_fails_fast_when_baseline_project_tests_are_red(self) -> None:
        root = self._temp_dir()
        self._write_failing_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/flag.py",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(2, exit_code)
        self.assertIn("Baseline tests failed.", err.getvalue())

    def test_restricts_mutations_to_requested_lines(self) -> None:
        root = self._temp_dir()
        self._write_two_mutation_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/pair.py",
                "--lines",
                "2",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(0, exit_code)
        self.assertIn("replace True with False", out.getvalue())
        self.assertNotIn("replace False with True", out.getvalue())
        self.assertIn("Summary: 1 killed, 0 survived, 1 total.", out.getvalue())

    def test_kills_timed_out_mutants(self) -> None:
        root = self._temp_dir()
        self._write_timeout_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/looping.py",
                "--timeout-factor",
                "1",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(0, exit_code)
        self.assertIn("replace not with removed not", out.getvalue())
        self.assertIn("timed out", out.getvalue())

    def test_reports_uncovered_sites_from_reused_coverage_and_skips_them(self) -> None:
        root = self._temp_dir()
        self._write_uncovered_project(root)
        self._write_coverage_xml(
            root,
            "demo/covered.py",
            [
                (2, 1),
                (5, 0),
            ],
        )
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/covered.py",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
                "--reuse-coverage",
            ]
        )

        self.assertEqual(0, exit_code)
        self.assertIn(
            "UNCOVERED demo/covered.py:5 replace False with True", out.getvalue()
        )
        self.assertIn("Coverage: 1 uncovered sites skipped.", out.getvalue())
        self.assertIn("Summary: 1 killed, 0 survived, 1 total.", out.getvalue())

    def test_warns_when_reuse_coverage_is_requested_without_report(self) -> None:
        root = self._temp_dir()
        self._write_passing_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/flag.py",
                "--reuse-coverage",
                "--lines",
                "2",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(0, exit_code)
        self.assertIn(
            "Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering.",
            err.getvalue(),
        )

    def test_runs_real_project_with_multiple_workers(self) -> None:
        root = self._temp_dir()
        self._write_two_mutation_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/pair.py",
                "--max-workers",
                "2",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(0, exit_code)
        self.assertIn("Summary: 2 killed, 0 survived, 2 total.", out.getvalue())

    def test_reports_verbose_progress_for_real_project(self) -> None:
        root = self._temp_dir()
        self._write_passing_project(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [
                "demo/flag.py",
                "--verbose",
                "--test-command",
                "python3 -m unittest discover -s tests -p 'test_*.py'",
            ]
        )

        self.assertEqual(0, exit_code)
        text = out.getvalue()
        self.assertIn(f"Baseline starting for {root}", text)
        self.assertIn("Baseline finished: exit=0 timedOut=false", text)
        self.assertIn("Running 1 mutations with 1 workers.", text)
        self.assertIn("Worker 1 starting 1/1: replace True with False", text)
        self.assertIn("Worker 1 finished 1/1: KILLED", text)

    def test_scan_marks_changed_scopes_for_real_project(self) -> None:
        root = self._temp_dir()
        self._write_passing_project(root)
        out = StringIO()
        err = StringIO()
        app = Application(root, StringIO(), StringIO())
        analysis = app.catalog.analyze(str(Path(root, "demo", "flag.py")))
        app.manifest_store.write("demo/flag.py", analysis)
        Path(root, "demo", "flag.py").write_text(
            "def enabled():\n    return False\n", encoding="utf-8"
        )

        exit_code = Application(root, out, err).execute(["demo/flag.py", "--scan"])

        self.assertEqual(0, exit_code)
        self.assertIn("* demo/flag.py:2 replace False with True", out.getvalue())
        self.assertIn(
            "* indicates a scope that differs from the stored manifest.", out.getvalue()
        )

    def _write_passing_project(self, root: str) -> None:
        Path(root, "demo").mkdir(parents=True, exist_ok=True)
        Path(root, "tests").mkdir(parents=True, exist_ok=True)
        Path(root, "demo", "flag.py").write_text(
            "def enabled():\n    return True\n", encoding="utf-8"
        )
        Path(root, "tests", "test_flag.py").write_text(
            "import unittest\n\n"
            "from demo.flag import enabled\n\n"
            "class FlagTest(unittest.TestCase):\n"
            "    def test_enabled(self):\n"
            "        self.assertTrue(enabled())\n\n"
            "if __name__ == '__main__':\n"
            "    unittest.main()\n",
            encoding="utf-8",
        )

    def _write_failing_project(self, root: str) -> None:
        self._write_passing_project(root)
        Path(root, "tests", "test_flag.py").write_text(
            "import unittest\n\n"
            "from demo.flag import enabled\n\n"
            "class FlagTest(unittest.TestCase):\n"
            "    def test_enabled(self):\n"
            "        self.assertFalse(enabled())\n\n"
            "if __name__ == '__main__':\n"
            "    unittest.main()\n",
            encoding="utf-8",
        )

    def _write_two_mutation_project(self, root: str) -> None:
        Path(root, "demo").mkdir(parents=True, exist_ok=True)
        Path(root, "tests").mkdir(parents=True, exist_ok=True)
        Path(root, "demo", "pair.py").write_text(
            "def first():\n    return True\n\ndef second():\n    return False\n",
            encoding="utf-8",
        )
        Path(root, "tests", "test_pair.py").write_text(
            "import unittest\n\n"
            "from demo.pair import first, second\n\n"
            "class PairTest(unittest.TestCase):\n"
            "    def test_pair(self):\n"
            "        self.assertTrue(first())\n"
            "        self.assertFalse(second())\n\n"
            "if __name__ == '__main__':\n"
            "    unittest.main()\n",
            encoding="utf-8",
        )

    def _write_timeout_project(self, root: str) -> None:
        Path(root, "demo").mkdir(parents=True, exist_ok=True)
        Path(root, "tests").mkdir(parents=True, exist_ok=True)
        Path(root, "demo", "looping.py").write_text(
            "def finishes(blocked):\n"
            "    while not blocked:\n"
            "        pass\n"
            "    return True\n",
            encoding="utf-8",
        )
        Path(root, "tests", "test_looping.py").write_text(
            "import unittest\n\n"
            "from demo.looping import finishes\n\n"
            "class LoopingTest(unittest.TestCase):\n"
            "    def test_finishes_when_initially_blocked(self):\n"
            "        self.assertTrue(finishes(True))\n\n"
            "if __name__ == '__main__':\n"
            "    unittest.main()\n",
            encoding="utf-8",
        )

    def _write_uncovered_project(self, root: str) -> None:
        Path(root, "demo").mkdir(parents=True, exist_ok=True)
        Path(root, "tests").mkdir(parents=True, exist_ok=True)
        Path(root, "demo", "covered.py").write_text(
            "def exercised():\n    return True\n\n"
            "def not_exercised():\n    return False\n",
            encoding="utf-8",
        )
        Path(root, "tests", "test_covered.py").write_text(
            "import unittest\n\n"
            "from demo.covered import exercised\n\n"
            "class CoveredTest(unittest.TestCase):\n"
            "    def test_exercised(self):\n"
            "        self.assertTrue(exercised())\n\n"
            "if __name__ == '__main__':\n"
            "    unittest.main()\n",
            encoding="utf-8",
        )

    def _write_coverage_xml(
        self, root: str, filename: str, lines: list[tuple[int, int]]
    ) -> None:
        Path(root, ".mutate", "coverage").mkdir(parents=True, exist_ok=True)
        line_xml = "".join(
            f'<line number="{line}" hits="{hits}"/>' for line, hits in lines
        )
        Path(root, ".mutate", "coverage", "coverage.xml").write_text(
            '<coverage><packages><package name="demo"><classes>'
            f'<class name="demo.covered" filename="{filename}"><lines>{line_xml}</lines></class>'
            "</classes></package></packages></coverage>",
            encoding="utf-8",
        )

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()
