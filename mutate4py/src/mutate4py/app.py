from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import TextIO
from concurrent.futures import ThreadPoolExecutor, as_completed
import importlib.util
import os
import sys

from mutate4py.analysis import MutationCatalog
from mutate4py.cli import parse_args
from mutate4py.coverage import parse_coverage_xml
from mutate4py.exec import ProcessCommandExecutor, ProcessTestCommandExecutor, TestRun
from mutate4py.manifest import ManifestStore
from mutate4py.model import CoverageReport
from mutate4py.workspace import prepare_worker_roots


BASELINE_TIMEOUT_MILLIS = 300000


def usage_text() -> str:
    return (
        "Usage: mutate4py <file.py> [options]\n\n"
        "Examples:\n"
        "  mutate4py demo/sample.py\n"
        "  mutate4py demo/sample.py --scan\n"
        "  mutate4py demo/sample.py --update-manifest\n"
        "  mutate4py demo/sample.py --lines 12,18\n"
    )


class Application:
    def __init__(
        self,
        workspace_root: str,
        out: TextIO,
        err: TextIO,
        test_executor=None,
        coverage_provider=None,
        progress_reporter=None,
    ) -> None:
        self.workspace_root = workspace_root
        self.out = out
        self.err = err
        self.catalog = MutationCatalog()
        self.manifest_store = ManifestStore(workspace_root)
        self.test_executor = test_executor or ProcessTestCommandExecutor()
        self.coverage_provider = coverage_provider or DefaultCoverageProvider(
            self.test_executor
        )
        self.progress_reporter = progress_reporter or NoOpProgressReporter()

    def execute(self, args: list[str]) -> int:
        try:
            parsed = parse_args(args)
        except ValueError as exc:
            self.out.write(usage_text())
            self.err.write(f"{exc}\n")
            return 1

        if parsed.mode.value == "help":
            self.out.write(usage_text())
            return 0

        progress_reporter = self.progress_reporter
        if parsed.verbose and isinstance(progress_reporter, NoOpProgressReporter):
            progress_reporter = PrintStreamProgressReporter(self.out)

        file_path = self._resolve_source_file(parsed.file_args[0])
        analysis = self.catalog.analyze(file_path)
        test_executor = self._executor_for(parsed.test_command)
        coverage_provider = self._coverage_provider_for(
            parsed.test_command, test_executor
        )

        if parsed.scan:
            changed_scope_ids: set[str] = set()
            changed = self.manifest_store.changed_scopes(parsed.file_args[0], analysis)
            if changed.manifest_present and changed.module_hash_changed:
                changed_scope_ids = changed.all_scope_ids()
            self._render_scan(parsed.file_args[0], analysis, changed_scope_ids)
            return 0

        if parsed.update_manifest:
            self.manifest_store.write(parsed.file_args[0], analysis)
            self.out.write(f"Updated manifest for {parsed.file_args[0]}\n")
            return 0

        coverage_run = coverage_provider.generate_coverage(
            self.workspace_root, parsed.reuse_coverage
        )
        progress_reporter.baseline_starting(self.workspace_root)
        if coverage_run.reused and coverage_run.report_available:
            self.err.write("Reusing existing coverage data; coverage may be stale.\n")
        elif coverage_run.reused and not coverage_run.report_available:
            self.err.write(
                "Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering.\n"
            )
        baseline = coverage_run.baseline
        progress_reporter.baseline_finished(baseline)
        if baseline.exit_code != 0:
            self.err.write("Baseline tests failed.\n")
            return 2

        selected_sites = list(analysis.sites)
        manifest_present = False
        module_hash_changed = False
        changed_mutation_sites = 0
        differential_surface_area = 0
        manifest_violating_surface_area = 0
        if parsed.since_last_run and not parsed.mutate_all:
            changed = self.manifest_store.changed_scopes(parsed.file_args[0], analysis)
            manifest_present = changed.manifest_present
            module_hash_changed = changed.module_hash_changed
            scope_ids = changed.all_scope_ids()
            if changed.manifest_present:
                changed_mutation_sites = len(
                    [site for site in analysis.sites if site.scope_id in scope_ids]
                )
                differential_surface_area = len(
                    [
                        site
                        for site in analysis.sites
                        if site.scope_id in changed.unregistered_scope_ids
                    ]
                )
                manifest_violating_surface_area = len(
                    [
                        site
                        for site in analysis.sites
                        if site.scope_id in changed.manifest_violation_scope_ids
                    ]
                )
                selected_sites = [
                    site for site in selected_sites if site.scope_id in scope_ids
                ]

        if parsed.lines:
            selected_sites = [
                site for site in selected_sites if site.line in parsed.lines
            ]

        covered_sites = list(selected_sites)
        uncovered_sites = []
        if coverage_run.report_available:
            covered_sites = []
            uncovered_sites = []
            for site in selected_sites:
                if coverage_run.report.covers(
                    parsed.file_args[0], site.line
                ) or coverage_run.report.covers(file_path, site.line):
                    covered_sites.append(site)
                else:
                    uncovered_sites.append(site)

        self.out.write(f"Baseline tests passed in {baseline.duration_millis} ms.\n")
        self.out.write(f"Total mutation sites: {len(analysis.sites)}\n")
        self.out.write(f"Covered mutation sites: {len(covered_sites)}\n")
        self.out.write(f"Uncovered mutation sites: {len(uncovered_sites)}\n")
        self.out.write(f"Changed mutation sites: {changed_mutation_sites}\n")
        self.out.write(f"Manifest exists: {_bool_text(manifest_present)}\n")
        self.out.write(f"Module hash changed: {_bool_text(module_hash_changed)}\n")
        self.out.write(f"Differential surface area: {differential_surface_area}\n")
        self.out.write(
            f"Manifest-violating surface area: {manifest_violating_surface_area}\n"
        )
        if len(covered_sites) > parsed.mutation_warning:
            self.out.write(
                f"WARNING: Found {len(covered_sites)} mutations. Consider splitting this module.\n"
            )

        for site in uncovered_sites:
            self.out.write(
                f"UNCOVERED {parsed.file_args[0]}:{site.line} {site.description}\n"
            )

        if not covered_sites:
            if uncovered_sites:
                self.out.write(
                    f"Coverage: {len(uncovered_sites)} uncovered sites skipped.\n"
                )
            self.out.write("No mutations need testing.\n")
            self.manifest_store.write(parsed.file_args[0], analysis)
            return 0

        timeout_millis = max(1000, baseline.duration_millis * parsed.timeout_factor)
        results = self._run_mutations(
            file_path,
            covered_sites,
            timeout_millis,
            test_executor,
            parsed.max_workers,
            progress_reporter,
        )

        killed = 0
        survived = 0
        for site, result in zip(covered_sites, results):
            status = "KILLED" if result.exit_code != 0 else "SURVIVED"
            if status == "KILLED":
                killed += 1
            else:
                survived += 1
            suffix = " timed out" if result.timed_out else ""
            self.out.write(
                f"{status} {parsed.file_args[0]}:{site.line} {site.description}{suffix}\n"
            )

        self.out.write(f"Coverage: {len(uncovered_sites)} uncovered sites skipped.\n")

        self.out.write(
            f"Summary: {killed} killed, {survived} survived, {len(results)} total.\n"
        )
        if survived > 0:
            return 3
        self.manifest_store.write(parsed.file_args[0], analysis)
        return 0

    def _resolve_source_file(self, file_arg: str) -> str:
        path = Path(self.workspace_root) / file_arg
        return str(path)

    def _render_scan(
        self, file_arg: str, analysis, changed_scope_ids: set[str]
    ) -> None:
        self.out.write(f"Scan: {len(analysis.sites)} mutation sites in {file_arg}\n")
        for site in analysis.sites:
            prefix = "* " if site.scope_id in changed_scope_ids else "  "
            self.out.write(f"{prefix}{file_arg}:{site.line} {site.description}\n")
        if changed_scope_ids:
            self.out.write(
                "* indicates a scope that differs from the stored manifest.\n"
            )

    def _run_mutation(
        self,
        file_path: str,
        site,
        timeout_millis: int,
        test_executor,
        workspace_root: str,
    ) -> TestRun:
        original = Path(file_path).read_text(encoding="utf-8")
        mutated = original[: site.start] + site.replacement_text + original[site.end :]
        Path(file_path).write_text(mutated, encoding="utf-8")
        try:
            return test_executor.run_tests(workspace_root, timeout_millis)
        finally:
            Path(file_path).write_text(original, encoding="utf-8")

    def _run_mutations(
        self,
        file_path: str,
        sites,
        timeout_millis: int,
        test_executor,
        max_workers: int,
        progress_reporter,
    ) -> list[TestRun]:
        worker_count = max(1, min(max_workers, len(sites)))
        progress_reporter.run_starting(len(sites), worker_count)
        if worker_count == 1:
            results = []
            for index, site in enumerate(sites, start=1):
                progress_reporter.mutation_starting(
                    1, index, len(sites), site.description
                )
                result = self._run_mutation(
                    file_path, site, timeout_millis, test_executor, self.workspace_root
                )
                progress_reporter.mutation_finished(
                    1, index, len(sites), result.exit_code != 0
                )
                results.append(result)
            return results

        relative_path = Path(file_path).relative_to(self.workspace_root)
        workspaces = prepare_worker_roots(self.workspace_root, worker_count)
        try:
            futures = []
            with ThreadPoolExecutor(max_workers=worker_count) as executor:
                for index, site in enumerate(sites):
                    worker_root = Path(workspaces.worker_roots[index % worker_count])
                    worker_file = worker_root / relative_path
                    worker_index = (index % worker_count) + 1
                    progress_reporter.mutation_starting(
                        worker_index, index + 1, len(sites), site.description
                    )
                    futures.append(
                        executor.submit(
                            self._run_mutation,
                            str(worker_file),
                            site,
                            timeout_millis,
                            test_executor,
                            str(worker_root),
                        )
                    )
                ordered: list[TestRun | None] = [None] * len(futures)
                for index, future in enumerate(futures):
                    result = future.result()
                    ordered[index] = result
                    progress_reporter.mutation_finished(
                        (index % worker_count) + 1,
                        index + 1,
                        len(sites),
                        result.exit_code != 0,
                    )
            return [result for result in ordered if result is not None]
        finally:
            workspaces.close()

    def _executor_for(self, test_command: str | None):
        if test_command and hasattr(self.test_executor, "with_command"):
            return self.test_executor.with_command(test_command)
        return self.test_executor

    def _coverage_provider_for(self, test_command: str | None, test_executor):
        if test_command and isinstance(self.coverage_provider, DefaultCoverageProvider):
            return DefaultCoverageProvider(test_executor, allow_generation=False)
        return self.coverage_provider


@dataclass(frozen=True)
class CoverageRun:
    baseline: TestRun
    report: CoverageReport
    report_available: bool
    reused: bool = False


class DefaultCoverageProvider:
    def __init__(
        self,
        test_executor,
        command_executor=None,
        coverage_available: bool | None = None,
        allow_generation: bool = True,
    ) -> None:
        self.test_executor = test_executor
        self.command_executor = command_executor or ProcessCommandExecutor()
        self.coverage_available = (
            _coverage_available() if coverage_available is None else coverage_available
        )
        self.allow_generation = allow_generation

    def generate_coverage(self, project_root: str, reuse: bool = False) -> CoverageRun:
        baseline = self.test_executor.run_tests(project_root, BASELINE_TIMEOUT_MILLIS)
        if reuse:
            coverage_xml = Path(project_root, ".mutate", "coverage", "coverage.xml")
            if coverage_xml.exists():
                return CoverageRun(
                    baseline=baseline,
                    report=parse_coverage_xml(str(coverage_xml)),
                    report_available=True,
                    reused=True,
                )
            return CoverageRun(
                baseline=baseline,
                report=CoverageReport({}),
                report_available=False,
                reused=True,
            )
        if not self.allow_generation or not self.coverage_available:
            return CoverageRun(
                baseline=baseline,
                report=CoverageReport({}),
                report_available=False,
            )

        coverage_xml = Path(project_root, ".mutate", "coverage", "coverage.xml")
        coverage_xml.parent.mkdir(parents=True, exist_ok=True)
        if coverage_xml.exists():
            coverage_xml.unlink()
        coverage_command = [
            sys.executable,
            "-m",
            "coverage",
            "run",
            "-m",
            "unittest",
            "discover",
            "-s",
            "tests",
            "-p",
            "test_*.py",
        ]
        result = self.command_executor.run(
            coverage_command,
            project_root,
            BASELINE_TIMEOUT_MILLIS,
        )
        if result.exit_code == 0:
            xml_result = self.command_executor.run(
                [
                    sys.executable,
                    "-m",
                    "coverage",
                    "xml",
                    "-o",
                    os.fspath(coverage_xml),
                ],
                project_root,
                BASELINE_TIMEOUT_MILLIS,
            )
            if xml_result.exit_code == 0 and coverage_xml.exists():
                return CoverageRun(
                    baseline=baseline,
                    report=parse_coverage_xml(str(coverage_xml)),
                    report_available=True,
                )
        return CoverageRun(
            baseline=baseline,
            report=CoverageReport({}),
            report_available=False,
        )


def _bool_text(value: bool) -> str:
    return "true" if value else "false"


class NoOpProgressReporter:
    def baseline_starting(self, project_root: str) -> None:
        pass

    def baseline_finished(self, baseline: TestRun) -> None:
        pass

    def run_starting(self, total_mutations: int, worker_count: int) -> None:
        pass

    def mutation_starting(
        self, worker_index: int, order: int, total_jobs: int, description: str
    ) -> None:
        pass

    def mutation_finished(
        self, worker_index: int, order: int, total_jobs: int, killed: bool
    ) -> None:
        pass


class PrintStreamProgressReporter:
    def __init__(self, out: TextIO) -> None:
        self.out = out

    def baseline_starting(self, project_root: str) -> None:
        self.out.write(f"Baseline starting for {project_root}\n")

    def baseline_finished(self, baseline: TestRun) -> None:
        self.out.write(
            "Baseline finished: "
            f"exit={baseline.exit_code} timedOut={_bool_text(baseline.timed_out)} duration={baseline.duration_millis} ms\n"
        )

    def run_starting(self, total_mutations: int, worker_count: int) -> None:
        self.out.write(
            f"Running {total_mutations} mutations with {worker_count} workers.\n"
        )

    def mutation_starting(
        self, worker_index: int, order: int, total_jobs: int, description: str
    ) -> None:
        self.out.write(
            f"Worker {worker_index} starting {order}/{total_jobs}: {description}\n"
        )

    def mutation_finished(
        self, worker_index: int, order: int, total_jobs: int, killed: bool
    ) -> None:
        status = "KILLED" if killed else "SURVIVED"
        self.out.write(
            f"Worker {worker_index} finished {order}/{total_jobs}: {status}\n"
        )


def _coverage_available() -> bool:
    return importlib.util.find_spec("coverage") is not None
