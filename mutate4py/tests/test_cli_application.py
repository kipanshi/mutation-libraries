from io import StringIO
from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.app import Application
from mutate4py.exec import TestRun
from mutate4py.model import CoverageReport


class CliApplicationTest(TestCase):
    def test_prints_help_and_exits_zero(self) -> None:
        out = StringIO()
        err = StringIO()

        exit_code = Application(self._temp_dir(), out, err).execute(["--help"])

        self.assertEqual(0, exit_code)
        self.assertIn("Usage:", out.getvalue())

    def test_prints_usage_for_invalid_arguments_and_exits_one(self) -> None:
        out = StringIO()
        err = StringIO()

        exit_code = Application(self._temp_dir(), out, err).execute(["bogus"])

        self.assertEqual(1, exit_code)
        self.assertIn("Usage:", out.getvalue())
        self.assertIn("mutate4py target must be a .py file", err.getvalue())

    def test_scans_mutation_sites_without_running_other_modes(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute([file_path, "--scan"])

        self.assertEqual(0, exit_code)
        self.assertIn("Scan: 2 mutation sites in demo/sample.py", out.getvalue())
        self.assertIn("demo/sample.py:2 replace True with False", out.getvalue())
        self.assertIn("demo/sample.py:5 replace == with !=", out.getvalue())
        self.assertEqual("", err.getvalue())

    def test_scan_marks_changed_scopes_when_manifest_differs(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        app = Application(root, StringIO(), StringIO())
        baseline = app.catalog.analyze(str(Path(root, file_path)))
        app.manifest_store.write(file_path, baseline)
        Path(root, file_path).write_text(
            "def truthy():\n"
            "    return False\n\n"
            "def same(left, right):\n"
            "    return left == right\n",
            encoding="utf-8",
        )
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute([file_path, "--scan"])

        self.assertEqual(0, exit_code)
        self.assertIn("* demo/sample.py:2 replace False with True", out.getvalue())
        self.assertIn(
            "* indicates a scope that differs from the stored manifest.", out.getvalue()
        )

    def test_updates_manifest_without_running_mutation_flow(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(root, out, err).execute(
            [file_path, "--update-manifest"]
        )

        self.assertEqual(0, exit_code)
        self.assertIn("Updated manifest for demo/sample.py", out.getvalue())
        self.assertEqual("", err.getvalue())
        self.assertTrue(
            Path(root, ".mutate", "manifests", "demo", "sample.py.json").exists()
        )

    def test_reports_matching_manifest_as_no_mutations_needed(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        app = Application(root, StringIO(), StringIO())
        analysis = app.catalog.analyze(str(Path(root, file_path)))
        app.manifest_store.write(file_path, analysis)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=StubTestExecutor([]),
        ).execute([file_path, "--since-last-run"])

        self.assertEqual(0, exit_code)
        self.assertIn("No mutations need testing.", out.getvalue())

    def test_since_last_run_without_manifest_runs_all_sites(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()
        executor = StubTestExecutor(
            [TestRun(1, "killed", 5, False), TestRun(1, "killed", 6, False)]
        )

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=executor,
        ).execute([file_path, "--since-last-run"])

        self.assertEqual(0, exit_code)
        self.assertIn("Summary: 2 killed, 0 survived, 2 total.", out.getvalue())
        self.assertEqual(2, executor.invocations)

    def test_mutate_all_ignores_matching_manifest(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        app = Application(root, StringIO(), StringIO())
        analysis = app.catalog.analyze(str(Path(root, file_path)))
        app.manifest_store.write(file_path, analysis)
        out = StringIO()
        err = StringIO()
        executor = StubTestExecutor(
            [TestRun(1, "killed", 5, False), TestRun(1, "killed", 6, False)]
        )

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=executor,
        ).execute([file_path, "--mutate-all"])

        self.assertEqual(0, exit_code)
        self.assertNotIn("No mutations need testing.", out.getvalue())
        self.assertIn("Summary: 2 killed, 0 survived, 2 total.", out.getvalue())
        self.assertEqual(2, executor.invocations)

    def test_stops_when_baseline_tests_fail(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(1, "baseline failed", 10, False),
                CoverageReport({}),
                False,
            ),
            test_executor=StubTestExecutor([]),
        ).execute([file_path])

        self.assertEqual(2, exit_code)
        self.assertIn("Baseline tests failed.", err.getvalue())

    def test_returns_nonzero_when_any_mutation_survives(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=StubTestExecutor(
                [TestRun(1, "killed", 5, False), TestRun(0, "survived", 6, False)]
            ),
        ).execute([file_path, "--max-workers", "1"])

        self.assertEqual(3, exit_code)
        self.assertIn("KILLED demo/sample.py:2 replace True with False", out.getvalue())
        self.assertIn("SURVIVED demo/sample.py:5 replace == with !=", out.getvalue())

    def test_reports_surface_area_for_changed_and_unregistered_scopes(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        app = Application(root, StringIO(), StringIO())
        baseline = app.catalog.analyze(str(Path(root, file_path)))
        app.manifest_store.write(file_path, baseline)
        Path(root, file_path).write_text(
            "def truthy():\n"
            "    return False\n\n"
            "def same(left, right):\n"
            "    return left == right\n\n"
            "def brand_new():\n"
            "    return True\n",
            encoding="utf-8",
        )
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 8}}),
                True,
            ),
            test_executor=StubTestExecutor(
                [TestRun(1, "killed", 5, False), TestRun(1, "killed", 6, False)]
            ),
        ).execute([file_path, "--since-last-run"])

        self.assertEqual(0, exit_code)
        text = out.getvalue()
        for fragment in [
            "Baseline tests passed in 10 ms.",
            "Total mutation sites: 3",
            "Covered mutation sites: 2",
            "Uncovered mutation sites: 0",
            "Changed mutation sites: 2",
            "Manifest exists: true",
            "Module hash changed: true",
            "Differential surface area: 1",
            "Manifest-violating surface area: 1",
            "Summary: 2 killed, 0 survived, 2 total.",
        ]:
            self.assertIn(fragment, text)

    def test_warns_when_covered_mutation_sites_exceed_threshold(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=StubTestExecutor(
                [TestRun(1, "killed", 5, False), TestRun(1, "killed", 6, False)]
            ),
        ).execute([file_path, "--mutation-warning", "1"])

        self.assertEqual(0, exit_code)
        self.assertIn(
            "WARNING: Found 2 mutations. Consider splitting this module.",
            out.getvalue(),
        )

    def test_reports_uncovered_sites_and_skips_them(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2}}),
                True,
            ),
            test_executor=StubTestExecutor([TestRun(1, "killed", 5, False)]),
        ).execute([file_path])

        self.assertEqual(0, exit_code)
        self.assertIn("UNCOVERED demo/sample.py:5 replace == with !=", out.getvalue())
        self.assertIn("Coverage: 1 uncovered sites skipped.", out.getvalue())
        self.assertIn("Summary: 1 killed, 0 survived, 1 total.", out.getvalue())

    def test_reports_no_mutations_needed_when_all_sites_are_uncovered(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({}),
                True,
            ),
            test_executor=StubTestExecutor([]),
        ).execute([file_path])

        self.assertEqual(0, exit_code)
        self.assertIn(
            "UNCOVERED demo/sample.py:2 replace True with False", out.getvalue()
        )
        self.assertIn("UNCOVERED demo/sample.py:5 replace == with !=", out.getvalue())
        self.assertIn("Coverage: 2 uncovered sites skipped.", out.getvalue())
        self.assertIn("No mutations need testing.", out.getvalue())

    def test_reuses_existing_coverage_and_warns_it_may_be_stale(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        self._write_coverage_xml(root, file_path, [(2, 1), (5, 1)])
        out = StringIO()
        err = StringIO()
        executor = StubTestExecutor(
            [
                TestRun(0, "baseline ok", 10, False),
                TestRun(1, "killed", 5, False),
                TestRun(1, "killed", 6, False),
            ]
        )

        exit_code = Application(root, out, err, test_executor=executor).execute(
            [file_path, "--reuse-coverage"]
        )

        self.assertEqual(0, exit_code)
        self.assertIn(
            "Reusing existing coverage data; coverage may be stale.", err.getvalue()
        )
        self.assertIn("Summary: 2 killed, 0 survived, 2 total.", out.getvalue())

    def test_warns_when_reuse_requested_without_coverage(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            test_executor=StubTestExecutor(
                [TestRun(0, "baseline ok", 10, False), TestRun(1, "killed", 5, False)]
            ),
        ).execute([file_path, "--reuse-coverage", "--lines", "2"])

        self.assertEqual(0, exit_code)
        self.assertIn(
            "Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering.",
            err.getvalue(),
        )

    def test_updates_manifest_after_clean_run(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=StubTestExecutor(
                [TestRun(1, "killed", 5, False), TestRun(1, "killed", 6, False)]
            ),
        ).execute([file_path])

        self.assertEqual(0, exit_code)
        self.assertTrue(
            Path(root, ".mutate", "manifests", "demo", "sample.py.json").exists()
        )

    def test_runs_mutations_with_multiple_workers(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        original = Path(root, file_path).read_text(encoding="utf-8")
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2, 5}}),
                True,
            ),
            test_executor=StubTestExecutor(
                [TestRun(1, "killed", 5, False), TestRun(1, "killed", 6, False)]
            ),
        ).execute([file_path, "--max-workers", "2"])

        self.assertEqual(0, exit_code)
        self.assertIn("Summary: 2 killed, 0 survived, 2 total.", out.getvalue())
        self.assertEqual(original, Path(root, file_path).read_text(encoding="utf-8"))

    def test_reports_mutation_progress_when_verbose(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root)
        out = StringIO()
        err = StringIO()

        exit_code = Application(
            root,
            out,
            err,
            coverage_provider=StubCoverageProvider(
                TestRun(0, "baseline ok", 10, False),
                CoverageReport({file_path: {2}}),
                True,
            ),
            test_executor=StubTestExecutor([TestRun(1, "killed", 5, False)]),
        ).execute([file_path, "--verbose", "--lines", "2"])

        self.assertEqual(0, exit_code)
        text = out.getvalue()
        for fragment in [
            f"Baseline starting for {root}",
            "Baseline finished: exit=0 timedOut=false duration=10 ms",
            "Running 1 mutations with 1 workers.",
            "Worker 1 starting 1/1: replace True with False",
            "Worker 1 finished 1/1: KILLED",
        ]:
            self.assertIn(fragment, text)

    def _write_source_file(self, root: str) -> str:
        path = Path(root, "demo", "sample.py")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            "def truthy():\n"
            "    return True\n\n"
            "def same(left, right):\n"
            "    return left == right\n",
            encoding="utf-8",
        )
        return "demo/sample.py"

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()

    def _write_coverage_xml(
        self, root: str, filename: str, lines: list[tuple[int, int]]
    ) -> None:
        Path(root, ".mutate", "coverage").mkdir(parents=True, exist_ok=True)
        line_xml = "".join(
            f'<line number="{line}" hits="{hits}"/>' for line, hits in lines
        )
        Path(root, ".mutate", "coverage", "coverage.xml").write_text(
            '<coverage><packages><package name="demo"><classes>'
            f'<class name="demo.sample" filename="{filename}"><lines>{line_xml}</lines></class>'
            "</classes></package></packages></coverage>",
            encoding="utf-8",
        )


class StubCoverageProvider:
    def __init__(
        self,
        baseline: TestRun,
        report: CoverageReport,
        report_available: bool,
        reused: bool = False,
    ):
        self.baseline = baseline
        self.report = report
        self.report_available = report_available
        self.reused = reused
        self.invocations = 0

    def generate_coverage(self, project_root: str, reuse: bool = False):
        self.invocations += 1
        return self


class StubTestExecutor:
    def __init__(self, results: list[TestRun]):
        self.results = list(results)
        self.invocations = 0

    def run_tests(self, project_root: str, timeout_millis: int) -> TestRun:
        self.invocations += 1
        return self.results.pop(0)
