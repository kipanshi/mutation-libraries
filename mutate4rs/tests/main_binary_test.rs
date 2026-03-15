use std::process::Command;

#[test]
fn binary_help_exit_code() {
    let output = Command::new(env!("CARGO_BIN_EXE_mutate4rs"))
        .arg("--help")
        .output()
        .unwrap();

    assert!(output.status.success());
    let stdout = String::from_utf8(output.stdout).unwrap();
    assert!(stdout.contains("Usage: mutate4rs <file.rs> [options]"));
}
