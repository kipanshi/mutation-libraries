use std::{
    fs,
    path::PathBuf,
    process::Command,
    time::{SystemTime, UNIX_EPOCH},
};

#[test]
fn mutates_a_real_cargo_project() {
    let root = temp_project_dir();
    write_passing_project(&root);

    let output = Command::new(env!("CARGO_BIN_EXE_mutate4rs"))
        .arg("src/lib.rs")
        .current_dir(&root)
        .output()
        .unwrap();

    assert!(
        output.status.success(),
        "stdout={} stderr={}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    let stdout = String::from_utf8(output.stdout).unwrap();
    assert!(
        stdout.contains("KILLED src/lib.rs:2 replace true with false"),
        "{stdout}"
    );
    assert!(
        stdout.contains("Summary: 1 killed, 0 survived, 1 total."),
        "{stdout}"
    );
}

#[test]
fn fails_fast_when_baseline_project_tests_are_red() {
    let root = temp_project_dir();
    write_failing_project(&root);

    let output = Command::new(env!("CARGO_BIN_EXE_mutate4rs"))
        .arg("src/lib.rs")
        .current_dir(&root)
        .output()
        .unwrap();

    assert_eq!(Some(2), output.status.code());
    let stderr = String::from_utf8(output.stderr).unwrap();
    assert!(stderr.contains("Baseline tests failed."), "{stderr}");
}

#[test]
fn reports_verbose_progress_for_real_project() {
    let root = temp_project_dir();
    write_passing_project(&root);

    let output = Command::new(env!("CARGO_BIN_EXE_mutate4rs"))
        .args(["src/lib.rs", "--verbose"])
        .current_dir(&root)
        .output()
        .unwrap();

    assert!(output.status.success());
    let stdout = String::from_utf8(output.stdout).unwrap();
    assert!(
        stdout.contains(&format!("Baseline starting for {}", root.display())),
        "{stdout}"
    );
    assert!(
        stdout.contains("Baseline finished: exit=0 timedOut=false"),
        "{stdout}"
    );
    assert!(
        stdout.contains("Running 1 mutations with 1 workers."),
        "{stdout}"
    );
    assert!(
        stdout.contains("Worker 1 starting 1/1: replace true with false"),
        "{stdout}"
    );
    assert!(stdout.contains("Worker 1 finished 1/1: KILLED"), "{stdout}");
}

#[test]
fn warns_when_reuse_coverage_is_requested_without_report() {
    let root = temp_project_dir();
    write_passing_project(&root);

    let output = Command::new(env!("CARGO_BIN_EXE_mutate4rs"))
        .args(["src/lib.rs", "--reuse-coverage", "--lines", "2"])
        .current_dir(&root)
        .output()
        .unwrap();

    assert!(output.status.success());
    let stderr = String::from_utf8(output.stderr).unwrap();
    assert!(stderr.contains("Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering."), "{stderr}");
}

fn temp_project_dir() -> PathBuf {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("mutate4rs-acceptance-{unique}"));
    fs::create_dir_all(&dir).unwrap();
    dir
}

fn write_passing_project(root: &PathBuf) {
    fs::create_dir_all(root.join("src")).unwrap();
    fs::create_dir_all(root.join("tests")).unwrap();
    fs::write(
        root.join("Cargo.toml"),
        "[package]\nname = \"demo_project\"\nversion = \"0.1.0\"\nedition = \"2024\"\n",
    )
    .unwrap();
    fs::write(
        root.join("src/lib.rs"),
        "pub fn enabled() -> bool {\n    true\n}\n",
    )
    .unwrap();
    fs::write(
        root.join("tests/enabled_test.rs"),
        "use demo_project::enabled;\n\n#[test]\nfn enabled_is_true() {\n    assert!(enabled());\n}\n",
    )
    .unwrap();
}

fn write_failing_project(root: &PathBuf) {
    write_passing_project(root);
    fs::write(
        root.join("tests/enabled_test.rs"),
        "use demo_project::enabled;\n\n#[test]\nfn enabled_is_false() {\n    assert!(!enabled());\n}\n",
    )
    .unwrap();
}
