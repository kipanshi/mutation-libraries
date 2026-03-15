use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use mutate4rs::parse_lcov;

#[test]
fn parses_covered_lines() {
    let file = temp_file("TN:\nSF:src/lib.rs\nDA:3,1\nDA:5,0\nend_of_record\n");
    let report = parse_lcov(&file).unwrap();
    assert!(report.covers("src/lib.rs", 3));
    assert!(!report.covers("src/lib.rs", 5));
}

#[test]
fn ignores_malformed_lines() {
    let file = temp_file("TN:\nSF:src/lib.rs\nDA:oops,nope\nend_of_record\n");
    let report = parse_lcov(&file).unwrap();
    assert!(!report.covers("src/lib.rs", 0));
}

fn temp_file(content: &str) -> PathBuf {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("mutate4rs-coverage-{unique}"));
    fs::create_dir_all(&dir).unwrap();
    let file = dir.join("coverage.info");
    fs::write(&file, content).unwrap();
    file
}
