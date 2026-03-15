use mutate4rs::ProcessTestCommandExecutor;

#[test]
fn captures_successful_test_run_output() {
    let result =
        ProcessTestCommandExecutor::new(vec!["sh".into(), "-c".into(), "printf ok".into()])
            .run_tests(&std::env::temp_dir(), 0)
            .unwrap();

    assert_eq!(0, result.exit_code);
    assert_eq!("ok", result.output);
    assert!(!result.timed_out);
}

#[test]
fn returns_timeout_exit_code_when_test_run_takes_too_long() {
    let result = ProcessTestCommandExecutor::new(vec!["sh".into(), "-c".into(), "sleep 1".into()])
        .run_tests(&std::env::temp_dir(), 10)
        .unwrap();

    assert_eq!(124, result.exit_code);
    assert!(result.timed_out);
}

#[test]
fn starts_shell_command_override_in_target_directory() {
    let result = ProcessTestCommandExecutor::new(vec![])
        .with_command("printf ok")
        .run_tests(&std::env::temp_dir(), 0)
        .unwrap();

    assert_eq!(0, result.exit_code);
    assert_eq!("ok", result.output);
    assert!(!result.timed_out);
}
