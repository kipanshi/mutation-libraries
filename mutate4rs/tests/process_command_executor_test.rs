use mutate4rs::ProcessCommandExecutor;

#[test]
fn captures_successful_command_output() {
    let result = ProcessCommandExecutor
        .run(&["sh", "-c", "printf ok"], &std::env::temp_dir(), 0)
        .unwrap();

    assert_eq!(0, result.exit_code);
    assert_eq!("ok", result.output);
    assert!(!result.timed_out);
}

#[test]
fn returns_timeout_exit_code_when_command_takes_too_long() {
    let result = ProcessCommandExecutor
        .run(&["sh", "-c", "sleep 1"], &std::env::temp_dir(), 10)
        .unwrap();

    assert_eq!(124, result.exit_code);
    assert!(result.timed_out);
}
