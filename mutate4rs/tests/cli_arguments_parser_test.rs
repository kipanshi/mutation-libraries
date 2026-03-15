use std::collections::BTreeSet;

use mutate4rs::{parse_args, CliMode};

#[test]
fn rejects_missing_file_argument() {
    let error = parse_args(&[]).unwrap_err();
    assert_eq!("mutate4rs requires exactly one Rust file", error);
}

#[test]
fn parses_single_explicit_file_argument() {
    let parsed = parse_args(&["demo/sample.rs"]).unwrap();
    assert_eq!(CliMode::ExplicitFiles, parsed.mode);
    assert_eq!(vec!["demo/sample.rs".to_string()], parsed.file_args);
    assert_eq!(BTreeSet::new(), parsed.lines);
    assert!(!parsed.scan);
    assert!(!parsed.update_manifest);
    assert!(!parsed.reuse_coverage);
    assert!(!parsed.since_last_run);
    assert!(!parsed.mutate_all);
    assert_eq!(10, parsed.timeout_factor);
    assert_eq!(50, parsed.mutation_warning);
    assert!(parsed.test_command.is_none());
    assert!(!parsed.verbose);
}

#[test]
fn parses_line_filter_and_timeout_factor() {
    let parsed = parse_args(&[
        "demo/sample.rs",
        "--lines",
        "12,18",
        "--timeout-factor",
        "15",
    ])
    .unwrap();
    assert_eq!(BTreeSet::from([12, 18]), parsed.lines);
    assert_eq!(15, parsed.timeout_factor);
}

#[test]
fn parses_help_mode() {
    let parsed = parse_args(&["--help"]).unwrap();
    assert_eq!(CliMode::Help, parsed.mode);
}

#[test]
fn rejects_non_rust_target() {
    let error = parse_args(&["bogus"]).unwrap_err();
    assert_eq!("mutate4rs target must be a .rs file", error);
}

#[test]
fn rejects_unknown_option() {
    let error = parse_args(&["demo/sample.rs", "--bogus"]).unwrap_err();
    assert_eq!("Unknown option: --bogus", error);
}

#[test]
fn rejects_lines_combined_with_since_last_run() {
    let error = parse_args(&["demo/sample.rs", "--lines", "5", "--since-last-run"]).unwrap_err();
    assert_eq!("--lines may not be combined with --since-last-run", error);
}

#[test]
fn rejects_blank_test_command() {
    let error = parse_args(&["demo/sample.rs", "--test-command", "   "]).unwrap_err();
    assert_eq!("--test-command must not be blank", error);
}
