use std::{
    fs,
    io::Cursor,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use mutate4rs::{
    Application, CoverageProvider, CoverageReport, CoverageRun, ManifestStore, MutationCatalog,
    TestExecutor, TestRun,
};

#[test]
fn prints_help_and_exits_zero() {
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::new(std::env::temp_dir(), &mut out, &mut err)
        .execute(&["--help"])
        .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("Usage:"));
}

#[test]
fn prints_usage_for_invalid_arguments_and_exits_one() {
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::new(std::env::temp_dir(), &mut out, &mut err)
        .execute(&["bogus"])
        .unwrap();

    assert_eq!(1, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    let error = String::from_utf8(err.into_inner()).unwrap();
    assert!(output.contains("Usage:"));
    assert!(error.contains("mutate4rs target must be a .rs file"));
}

#[test]
fn scans_mutation_sites_without_running_other_modes() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::new(root.clone(), &mut out, &mut err)
        .execute(&[&file_arg, "--scan"])
        .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("Scan: 2 mutation sites in src/lib.rs"));
    assert!(output.contains("src/lib.rs:1 replace true with false"));
    assert!(output.contains("src/lib.rs:2 replace == with !="));
}

#[test]
fn updates_manifest_without_running_mutation_flow() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::new(root.clone(), &mut out, &mut err)
        .execute(&[&file_arg, "--update-manifest"])
        .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("Updated manifest for src/lib.rs"));
    assert!(root
        .join(".mutate")
        .join("manifests")
        .join("src")
        .join("lib.rs.json")
        .exists());
}

#[test]
fn scan_marks_changed_scopes_when_manifest_differs() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n");
    let analysis = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();
    ManifestStore::new(root.clone())
        .write(&file_arg, &analysis)
        .unwrap();
    fs::write(root.join(&file_arg), "pub fn truthy() -> bool { false }\npub fn same(left: i32, right: i32) -> bool { left == right }\n").unwrap();
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::new(root, &mut out, &mut err)
        .execute(&[&file_arg, "--scan"])
        .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("* src/lib.rs:1 replace false with true"));
    assert!(output.contains("* indicates a scope that differs from the stored manifest."));
}

#[test]
fn stops_when_baseline_tests_fail() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 1,
                output: "baseline failed".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport::default(),
            false,
            false,
        )),
    )
    .execute(&[&file_arg])
    .unwrap();

    assert_eq!(2, exit);
    let error = String::from_utf8(err.into_inner()).unwrap();
    assert!(error.contains("Baseline tests failed."));
}

#[test]
fn returns_nonzero_when_any_mutation_survives() {
    let root = temp_dir();
    let file_arg = write_source_file(
        &root,
        "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n",
    );
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(
        file_arg.clone(),
        std::collections::BTreeSet::from([1usize, 2usize]),
    );

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 5,
                timed_out: false,
            },
            TestRun {
                exit_code: 0,
                output: "survived".into(),
                duration_millis: 6,
                timed_out: false,
            },
        ])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--max-workers", "1"])
    .unwrap();

    assert_eq!(3, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("KILLED src/lib.rs:1 replace true with false"));
    assert!(output.contains("SURVIVED src/lib.rs:2 replace == with !="));
}

#[test]
fn reports_uncovered_sites_and_skips_them() {
    let root = temp_dir();
    let file_arg = write_source_file(
        &root,
        "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n",
    );
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(file_arg.clone(), std::collections::BTreeSet::from([1usize]));

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![TestRun {
            exit_code: 1,
            output: "killed".into(),
            duration_millis: 5,
            timed_out: false,
        }])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("UNCOVERED src/lib.rs:2 replace == with !="));
    assert!(output.contains("Coverage: 1 uncovered sites skipped."));
}

#[test]
fn runs_mutations_with_multiple_workers() {
    let root = temp_dir();
    let file_arg = write_source_file(
        &root,
        "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n",
    );
    let original = fs::read_to_string(root.join(&file_arg)).unwrap();
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(
        file_arg.clone(),
        std::collections::BTreeSet::from([1usize, 2usize]),
    );

    let exit = Application::with_dependencies(
        root.clone(),
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 5,
                timed_out: false,
            },
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 6,
                timed_out: false,
            },
        ])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--max-workers", "2"])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("Summary: 2 killed, 0 survived, 2 total."));
    assert_eq!(original, fs::read_to_string(root.join(file_arg)).unwrap());
}

#[test]
fn reports_basic_run_progress_even_without_verbose() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(file_arg.clone(), std::collections::BTreeSet::from([1usize]));

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![TestRun {
            exit_code: 1,
            output: "killed".into(),
            duration_millis: 5,
            timed_out: false,
        }])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--lines", "1"])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(
        output.contains("Running 1 mutations with 1 workers."),
        "{output}"
    );
    assert!(!output.contains("Worker 1 starting"), "{output}");
}

#[test]
fn reports_verbose_progress_when_requested() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(file_arg.clone(), std::collections::BTreeSet::from([1usize]));

    let exit = Application::with_dependencies(
        root.clone(),
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![TestRun {
            exit_code: 1,
            output: "killed".into(),
            duration_millis: 5,
            timed_out: false,
        }])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--verbose", "--lines", "1"])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    for fragment in [
        format!("Baseline starting for {}", root.display()),
        "Baseline finished: exit=0 timedOut=false duration=10 ms".to_string(),
        "Running 1 mutations with 1 workers.".to_string(),
        "Worker 1 starting 1/1: replace true with false".to_string(),
        "Worker 1 finished 1/1: KILLED".to_string(),
    ] {
        assert!(output.contains(&fragment), "missing {fragment} in {output}");
    }
}

#[test]
fn warns_when_reuse_coverage_is_requested_without_report() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![TestRun {
            exit_code: 1,
            output: "killed".into(),
            duration_millis: 5,
            timed_out: false,
        }])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport::default(),
            false,
            true,
        )),
    )
    .execute(&[&file_arg, "--reuse-coverage", "--lines", "1"])
    .unwrap();

    assert_eq!(0, exit);
    let error = String::from_utf8(err.into_inner()).unwrap();
    assert!(error.contains("Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering."));
}

#[test]
fn warns_when_reusing_existing_coverage() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(file_arg.clone(), std::collections::BTreeSet::from([1usize]));

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![TestRun {
            exit_code: 1,
            output: "killed".into(),
            duration_millis: 5,
            timed_out: false,
        }])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            true,
        )),
    )
    .execute(&[&file_arg, "--reuse-coverage", "--lines", "1"])
    .unwrap();

    assert_eq!(0, exit);
    let error = String::from_utf8(err.into_inner()).unwrap();
    assert!(error.contains("Reusing existing coverage data; coverage may be stale."));
}

#[test]
fn reports_matching_manifest_as_no_mutations_needed() {
    let root = temp_dir();
    let file_arg = write_source_file(
        &root,
        "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n",
    );
    let analysis = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();
    ManifestStore::new(root.clone())
        .write(&file_arg, &analysis)
        .unwrap();
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(
        file_arg.clone(),
        std::collections::BTreeSet::from([1usize, 2usize]),
    );

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--since-last-run"])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("No mutations need testing."));
}

#[test]
fn mutate_all_ignores_matching_manifest() {
    let root = temp_dir();
    let file_arg = write_source_file(
        &root,
        "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n",
    );
    let analysis = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();
    ManifestStore::new(root.clone())
        .write(&file_arg, &analysis)
        .unwrap();
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(
        file_arg.clone(),
        std::collections::BTreeSet::from([1usize, 2usize]),
    );

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 5,
                timed_out: false,
            },
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 6,
                timed_out: false,
            },
        ])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--mutate-all"])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
    assert!(output.contains("Summary: 2 killed, 0 survived, 2 total."));
    assert!(!output.contains("No mutations need testing."));
}

#[test]
fn reports_surface_area_for_changed_and_unregistered_scopes() {
    let root = temp_dir();
    let file_arg = write_source_file(
        &root,
        "pub fn truthy() -> bool { true }\npub fn same(left: i32, right: i32) -> bool { left == right }\n",
    );
    let baseline = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();
    ManifestStore::new(root.clone())
        .write(&file_arg, &baseline)
        .unwrap();
    fs::write(
        root.join(&file_arg),
        "pub fn truthy() -> bool { false }\npub fn same(left: i32, right: i32) -> bool { left == right }\npub fn brand_new() -> bool { true }\n",
    )
    .unwrap();
    let mut out = Cursor::new(Vec::<u8>::new());
    let mut err = Cursor::new(Vec::<u8>::new());
    let mut covered = std::collections::BTreeMap::new();
    covered.insert(
        file_arg.clone(),
        std::collections::BTreeSet::from([1usize, 3usize]),
    );

    let exit = Application::with_dependencies(
        root,
        &mut out,
        &mut err,
        Box::new(StubTestExecutor::new(vec![
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 5,
                timed_out: false,
            },
            TestRun {
                exit_code: 1,
                output: "killed".into(),
                duration_millis: 6,
                timed_out: false,
            },
        ])),
        Box::new(StubCoverageProvider::new(
            TestRun {
                exit_code: 0,
                output: "baseline ok".into(),
                duration_millis: 10,
                timed_out: false,
            },
            CoverageReport { covered },
            true,
            false,
        )),
    )
    .execute(&[&file_arg, "--since-last-run"])
    .unwrap();

    assert_eq!(0, exit);
    let output = String::from_utf8(out.into_inner()).unwrap();
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
    ] {
        assert!(output.contains(fragment), "missing {fragment} in {output}");
    }
}

fn temp_dir() -> PathBuf {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("mutate4rs-app-{unique}"));
    fs::create_dir_all(&dir).unwrap();
    dir
}

fn write_source_file(root: &PathBuf, content: &str) -> String {
    let file_arg = "src/lib.rs".to_string();
    let path = root.join(&file_arg);
    fs::create_dir_all(path.parent().unwrap()).unwrap();
    fs::write(path, content).unwrap();
    file_arg
}

struct StubCoverageProvider {
    baseline: TestRun,
    report: CoverageReport,
    report_available: bool,
    reused: bool,
}

impl StubCoverageProvider {
    fn new(
        baseline: TestRun,
        report: CoverageReport,
        report_available: bool,
        reused: bool,
    ) -> Self {
        Self {
            baseline,
            report,
            report_available,
            reused,
        }
    }
}

impl CoverageProvider for StubCoverageProvider {
    fn generate_coverage(
        &self,
        _project_root: &std::path::Path,
        _reuse: bool,
    ) -> Result<CoverageRun, String> {
        Ok(CoverageRun {
            baseline: self.baseline.clone(),
            report: self.report.clone(),
            report_available: self.report_available,
            reused: self.reused,
        })
    }
}

struct StubTestExecutor {
    results: std::sync::Mutex<Vec<TestRun>>,
}

impl StubTestExecutor {
    fn new(results: Vec<TestRun>) -> Self {
        Self {
            results: std::sync::Mutex::new(results),
        }
    }
}

impl TestExecutor for StubTestExecutor {
    fn run_tests(
        &self,
        _project_root: &std::path::Path,
        _timeout_millis: u64,
    ) -> Result<TestRun, String> {
        self.results
            .lock()
            .unwrap()
            .drain(..1)
            .next()
            .ok_or_else(|| "no stub result available".to_string())
    }
}
