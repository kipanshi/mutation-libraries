use std::{
    io::Write,
    path::{Path, PathBuf},
};

use crate::{
    analysis::MutationCatalog,
    cli::parse_args,
    coverage::parse_lcov,
    exec::{ProcessTestCommandExecutor, TestRun},
    manifest::ManifestStore,
    model::{CoverageReport, SourceAnalysis},
    workspace::prepare_worker_roots,
};

const BASELINE_TIMEOUT_MILLIS: u64 = 300_000;

pub trait TestExecutor {
    fn run_tests(&self, project_root: &Path, timeout_millis: u64) -> Result<TestRun, String>;
}

impl TestExecutor for ProcessTestCommandExecutor {
    fn run_tests(&self, project_root: &Path, timeout_millis: u64) -> Result<TestRun, String> {
        ProcessTestCommandExecutor::run_tests(self, project_root, timeout_millis)
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CoverageRun {
    pub baseline: TestRun,
    pub report: CoverageReport,
    pub report_available: bool,
    pub reused: bool,
}

pub trait CoverageProvider {
    fn generate_coverage(&self, project_root: &Path, reuse: bool) -> Result<CoverageRun, String>;
}

pub struct DefaultCoverageProvider {
    test_executor: ProcessTestCommandExecutor,
}

impl DefaultCoverageProvider {
    pub fn new(test_executor: ProcessTestCommandExecutor) -> Self {
        Self { test_executor }
    }
}

impl CoverageProvider for DefaultCoverageProvider {
    fn generate_coverage(&self, project_root: &Path, reuse: bool) -> Result<CoverageRun, String> {
        let baseline = self
            .test_executor
            .run_tests(project_root, BASELINE_TIMEOUT_MILLIS)?;
        if reuse {
            let coverage_path = project_root
                .join(".mutate")
                .join("coverage")
                .join("lcov.info");
            if coverage_path.exists() {
                return Ok(CoverageRun {
                    baseline,
                    report: parse_lcov(&coverage_path)?,
                    report_available: true,
                    reused: true,
                });
            }
            return Ok(CoverageRun {
                baseline,
                report: CoverageReport::default(),
                report_available: false,
                reused: true,
            });
        }
        Ok(CoverageRun {
            baseline,
            report: CoverageReport::default(),
            report_available: false,
            reused: false,
        })
    }
}

pub struct Application<'a> {
    workspace_root: PathBuf,
    catalog: MutationCatalog,
    manifest_store: ManifestStore,
    test_executor: Box<dyn TestExecutor>,
    coverage_provider: Box<dyn CoverageProvider>,
    out: &'a mut dyn Write,
    err: &'a mut dyn Write,
}

impl<'a> Application<'a> {
    pub fn new(workspace_root: PathBuf, out: &'a mut dyn Write, err: &'a mut dyn Write) -> Self {
        let test_executor = ProcessTestCommandExecutor::new(vec![]);
        let coverage_provider = DefaultCoverageProvider::new(test_executor.clone());
        Self::with_dependencies(
            workspace_root,
            out,
            err,
            Box::new(test_executor),
            Box::new(coverage_provider),
        )
    }

    pub fn with_dependencies(
        workspace_root: PathBuf,
        out: &'a mut dyn Write,
        err: &'a mut dyn Write,
        test_executor: Box<dyn TestExecutor>,
        coverage_provider: Box<dyn CoverageProvider>,
    ) -> Self {
        Self {
            catalog: MutationCatalog,
            manifest_store: ManifestStore::new(workspace_root.clone()),
            workspace_root,
            test_executor,
            coverage_provider,
            out,
            err,
        }
    }

    pub fn execute(&mut self, args: &[&str]) -> Result<i32, String> {
        match parse_args(args) {
            Ok(parsed) if matches!(parsed.mode, crate::model::CliMode::Help) => {
                self.out
                    .write_all(usage_text().as_bytes())
                    .map_err(|err| err.to_string())?;
                Ok(0)
            }
            Ok(parsed) => {
                let file_arg = &parsed.file_args[0];
                let source_file = self.workspace_root.join(file_arg);
                let analysis = self.catalog.analyze(&source_file)?;

                if parsed.scan {
                    let changed = self.manifest_store.changed_scopes(file_arg, &analysis)?;
                    let changed_scope_ids =
                        if changed.manifest_present && changed.module_hash_changed {
                            changed.all_scope_ids()
                        } else {
                            Default::default()
                        };
                    self.render_scan(file_arg, &analysis, &changed_scope_ids)?;
                    return Ok(0);
                }

                if parsed.update_manifest {
                    self.manifest_store.write(file_arg, &analysis)?;
                    self.out
                        .write_all(format!("Updated manifest for {file_arg}\n").as_bytes())
                        .map_err(|err| err.to_string())?;
                    return Ok(0);
                }

                if parsed.verbose {
                    self.out
                        .write_all(
                            format!("Baseline starting for {}\n", self.workspace_root.display())
                                .as_bytes(),
                        )
                        .map_err(|err| err.to_string())?;
                }
                let coverage_run = self
                    .coverage_provider
                    .generate_coverage(&self.workspace_root, parsed.reuse_coverage)?;
                if coverage_run.reused && coverage_run.report_available {
                    self.err
                        .write_all(b"Reusing existing coverage data; coverage may be stale.\n")
                        .map_err(|err| err.to_string())?;
                } else if coverage_run.reused && !coverage_run.report_available {
                    self.err
                        .write_all(b"Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering.\n")
                        .map_err(|err| err.to_string())?;
                }
                let baseline = coverage_run.baseline;
                if parsed.verbose {
                    self.out
                        .write_all(
                            format!(
                                "Baseline finished: exit={} timedOut={} duration={} ms\n",
                                baseline.exit_code,
                                bool_text(baseline.timed_out),
                                baseline.duration_millis
                            )
                            .as_bytes(),
                        )
                        .map_err(|err| err.to_string())?;
                }
                if baseline.exit_code != 0 {
                    self.err
                        .write_all(b"Baseline tests failed.\n")
                        .map_err(|err| err.to_string())?;
                    return Ok(2);
                }

                let changed = if parsed.since_last_run && !parsed.mutate_all {
                    self.manifest_store.changed_scopes(file_arg, &analysis)?
                } else {
                    Default::default()
                };
                let scope_ids = changed.all_scope_ids();
                let mut selected_sites =
                    if parsed.since_last_run && !parsed.mutate_all && changed.manifest_present {
                        analysis
                            .sites
                            .iter()
                            .filter(|site| scope_ids.contains(&site.scope_id))
                            .cloned()
                            .collect::<Vec<_>>()
                    } else {
                        analysis.sites.clone()
                    };

                if !parsed.lines.is_empty() {
                    selected_sites.retain(|site| parsed.lines.contains(&site.line));
                }

                let mut covered_sites = selected_sites.clone();
                let mut uncovered_sites = Vec::new();
                if coverage_run.report_available {
                    covered_sites.clear();
                    for site in selected_sites {
                        if coverage_run.report.covers(file_arg, site.line)
                            || coverage_run
                                .report
                                .covers(&source_file.to_string_lossy(), site.line)
                        {
                            covered_sites.push(site);
                        } else {
                            uncovered_sites.push(site);
                        }
                    }
                }

                self.write_summary_header(
                    &baseline,
                    &analysis,
                    &covered_sites,
                    &uncovered_sites,
                    &changed,
                    parsed.mutation_warning,
                )?;
                for site in &uncovered_sites {
                    self.out
                        .write_all(
                            format!("UNCOVERED {file_arg}:{} {}\n", site.line, site.description)
                                .as_bytes(),
                        )
                        .map_err(|err| err.to_string())?;
                }
                if covered_sites.is_empty() {
                    if !uncovered_sites.is_empty() {
                        self.out
                            .write_all(
                                format!(
                                    "Coverage: {} uncovered sites skipped.\n",
                                    uncovered_sites.len()
                                )
                                .as_bytes(),
                            )
                            .map_err(|err| err.to_string())?;
                    }
                    self.out
                        .write_all(b"No mutations need testing.\n")
                        .map_err(|err| err.to_string())?;
                    self.manifest_store.write(file_arg, &analysis)?;
                    return Ok(0);
                }

                let timeout_millis = std::cmp::max(
                    1_000,
                    baseline.duration_millis as u64 * parsed.timeout_factor as u64,
                );
                let results = self.run_mutations(
                    &source_file,
                    &covered_sites,
                    timeout_millis,
                    parsed.max_workers,
                    parsed.verbose,
                )?;
                let mut killed = 0usize;
                let mut survived = 0usize;
                for (site, result) in covered_sites.iter().zip(results.iter()) {
                    let status = if result.exit_code != 0 {
                        killed += 1;
                        "KILLED"
                    } else {
                        survived += 1;
                        "SURVIVED"
                    };
                    let suffix = if result.timed_out { " timed out" } else { "" };
                    self.out
                        .write_all(
                            format!(
                                "{status} {file_arg}:{} {}{}\n",
                                site.line, site.description, suffix
                            )
                            .as_bytes(),
                        )
                        .map_err(|err| err.to_string())?;
                }
                self.out
                    .write_all(
                        format!(
                            "Coverage: {} uncovered sites skipped.\n",
                            uncovered_sites.len()
                        )
                        .as_bytes(),
                    )
                    .map_err(|err| err.to_string())?;
                self.out
                    .write_all(
                        format!(
                            "Summary: {killed} killed, {survived} survived, {} total.\n",
                            covered_sites.len()
                        )
                        .as_bytes(),
                    )
                    .map_err(|err| err.to_string())?;
                if survived > 0 {
                    return Ok(3);
                }
                self.manifest_store.write(file_arg, &analysis)?;
                Ok(0)
            }
            Err(error) => {
                self.out
                    .write_all(usage_text().as_bytes())
                    .map_err(|err| err.to_string())?;
                self.err
                    .write_all(format!("{error}\n").as_bytes())
                    .map_err(|err| err.to_string())?;
                Ok(1)
            }
        }
    }

    fn write_summary_header(
        &mut self,
        baseline: &TestRun,
        analysis: &SourceAnalysis,
        covered_sites: &[crate::model::MutationSite],
        uncovered_sites: &[crate::model::MutationSite],
        changed: &crate::model::ChangedScopes,
        mutation_warning: usize,
    ) -> Result<(), String> {
        let changed_scope_ids = changed.all_scope_ids();
        let changed_mutation_sites = analysis
            .sites
            .iter()
            .filter(|site| changed_scope_ids.contains(&site.scope_id))
            .count();
        let differential_surface_area = analysis
            .sites
            .iter()
            .filter(|site| changed.unregistered_scope_ids.contains(&site.scope_id))
            .count();
        let manifest_violating_surface_area = analysis
            .sites
            .iter()
            .filter(|site| {
                changed
                    .manifest_violation_scope_ids
                    .contains(&site.scope_id)
            })
            .count();

        self.out
            .write_all(
                format!(
                    "Baseline tests passed in {} ms.\n",
                    baseline.duration_millis
                )
                .as_bytes(),
            )
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(format!("Total mutation sites: {}\n", analysis.sites.len()).as_bytes())
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(format!("Covered mutation sites: {}\n", covered_sites.len()).as_bytes())
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(format!("Uncovered mutation sites: {}\n", uncovered_sites.len()).as_bytes())
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(format!("Changed mutation sites: {changed_mutation_sites}\n").as_bytes())
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(
                format!("Manifest exists: {}\n", bool_text(changed.manifest_present)).as_bytes(),
            )
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(
                format!(
                    "Module hash changed: {}\n",
                    bool_text(changed.module_hash_changed)
                )
                .as_bytes(),
            )
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(
                format!("Differential surface area: {differential_surface_area}\n").as_bytes(),
            )
            .map_err(|err| err.to_string())?;
        self.out
            .write_all(
                format!("Manifest-violating surface area: {manifest_violating_surface_area}\n")
                    .as_bytes(),
            )
            .map_err(|err| err.to_string())?;
        if covered_sites.len() > mutation_warning {
            self.out
                .write_all(
                    format!(
                        "WARNING: Found {} mutations. Consider splitting this module.\n",
                        covered_sites.len()
                    )
                    .as_bytes(),
                )
                .map_err(|err| err.to_string())?;
        }
        Ok(())
    }

    fn render_scan(
        &mut self,
        file_arg: &str,
        analysis: &crate::model::SourceAnalysis,
        changed_scope_ids: &std::collections::BTreeSet<String>,
    ) -> Result<(), String> {
        self.out
            .write_all(
                format!(
                    "Scan: {} mutation sites in {file_arg}\n",
                    analysis.sites.len()
                )
                .as_bytes(),
            )
            .map_err(|err| err.to_string())?;
        for site in &analysis.sites {
            let prefix = if changed_scope_ids.contains(&site.scope_id) {
                "* "
            } else {
                "  "
            };
            self.out
                .write_all(
                    format!("{prefix}{file_arg}:{} {}\n", site.line, site.description).as_bytes(),
                )
                .map_err(|err| err.to_string())?;
        }
        if !changed_scope_ids.is_empty() {
            self.out
                .write_all(b"* indicates a scope that differs from the stored manifest.\n")
                .map_err(|err| err.to_string())?;
        }
        Ok(())
    }

    fn run_mutation(
        &self,
        source_file: &Path,
        site: &crate::model::MutationSite,
        timeout_millis: u64,
        project_root: &Path,
    ) -> Result<TestRun, String> {
        let original = std::fs::read_to_string(source_file).map_err(|err| err.to_string())?;
        let mutated = format!(
            "{}{}{}",
            &original[..site.start],
            site.replacement_text,
            &original[site.end..]
        );
        std::fs::write(source_file, mutated).map_err(|err| err.to_string())?;
        let result = self.test_executor.run_tests(project_root, timeout_millis);
        std::fs::write(source_file, original).map_err(|err| err.to_string())?;
        result
    }

    fn run_mutations(
        &mut self,
        source_file: &Path,
        sites: &[crate::model::MutationSite],
        timeout_millis: u64,
        max_workers: usize,
        verbose: bool,
    ) -> Result<Vec<TestRun>, String> {
        let worker_count = std::cmp::max(1, std::cmp::min(max_workers, sites.len()));
        if verbose || !sites.is_empty() {
            write!(
                self.out,
                "Running {} mutations with {} workers.\n",
                sites.len(),
                worker_count
            )
            .map_err(|err| err.to_string())?;
        }
        if worker_count == 1 {
            let mut results = Vec::new();
            for (index, site) in sites.iter().enumerate() {
                if verbose {
                    write!(
                        self.out,
                        "Worker 1 starting {}/{}: {}\n",
                        index + 1,
                        sites.len(),
                        site.description
                    )
                    .map_err(|err| err.to_string())?;
                }
                let result =
                    self.run_mutation(source_file, site, timeout_millis, &self.workspace_root)?;
                if verbose {
                    write!(
                        self.out,
                        "Worker 1 finished {}/{}: {}\n",
                        index + 1,
                        sites.len(),
                        if result.exit_code != 0 {
                            "KILLED"
                        } else {
                            "SURVIVED"
                        }
                    )
                    .map_err(|err| err.to_string())?;
                }
                results.push(result);
            }
            return Ok(results);
        }

        let relative = source_file
            .strip_prefix(&self.workspace_root)
            .map_err(|err| err.to_string())?
            .to_path_buf();
        let workspaces = prepare_worker_roots(&self.workspace_root, worker_count)?;
        let mut results = Vec::new();
        for (index, site) in sites.iter().enumerate() {
            let worker_root = PathBuf::from(&workspaces.worker_roots[index % worker_count]);
            let worker_file = worker_root.join(&relative);
            let worker_index = (index % worker_count) + 1;
            if verbose {
                write!(
                    self.out,
                    "Worker {} starting {}/{}: {}\n",
                    worker_index,
                    index + 1,
                    sites.len(),
                    site.description
                )
                .map_err(|err| err.to_string())?;
            }
            let result = self.run_mutation(&worker_file, site, timeout_millis, &worker_root)?;
            if verbose {
                write!(
                    self.out,
                    "Worker {} finished {}/{}: {}\n",
                    worker_index,
                    index + 1,
                    sites.len(),
                    if result.exit_code != 0 {
                        "KILLED"
                    } else {
                        "SURVIVED"
                    }
                )
                .map_err(|err| err.to_string())?;
            }
            results.push(result);
        }
        workspaces.close()?;
        Ok(results)
    }
}

pub fn run(
    args: &[&str],
    workspace_root: PathBuf,
    out: &mut dyn Write,
    err: &mut dyn Write,
) -> i32 {
    Application::new(workspace_root, out, err)
        .execute(args)
        .unwrap_or(1)
}

fn usage_text() -> &'static str {
    "Usage: mutate4rs <file.rs> [options]\n\nExamples:\n  mutate4rs src/lib.rs\n  mutate4rs src/lib.rs --scan\n  mutate4rs src/lib.rs --update-manifest\n  mutate4rs src/lib.rs --lines 12,18\n"
}

fn bool_text(value: bool) -> &'static str {
    if value {
        "true"
    } else {
        "false"
    }
}
