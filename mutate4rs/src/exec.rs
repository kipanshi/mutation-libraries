use std::{
    path::Path,
    process::{Command, Stdio},
    thread,
    time::{Duration, Instant},
};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CommandResult {
    pub exit_code: i32,
    pub output: String,
    pub duration_millis: u128,
    pub timed_out: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TestRun {
    pub exit_code: i32,
    pub output: String,
    pub duration_millis: u128,
    pub timed_out: bool,
}

pub struct ProcessCommandExecutor;

impl ProcessCommandExecutor {
    pub fn run(
        &self,
        command: &[&str],
        working_directory: &Path,
        timeout_millis: u64,
    ) -> Result<CommandResult, String> {
        let Some(program) = command.first() else {
            return Err("command must not be empty".to_string());
        };
        let start = Instant::now();
        let mut child = Command::new(program)
            .args(&command[1..])
            .current_dir(working_directory)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|err| err.to_string())?;

        if timeout_millis > 0 {
            let timeout = Duration::from_millis(timeout_millis);
            loop {
                if let Some(_status) = child.try_wait().map_err(|err| err.to_string())? {
                    let output = child.wait_with_output().map_err(|err| err.to_string())?;
                    return Ok(CommandResult {
                        exit_code: output.status.code().unwrap_or(1),
                        output: String::from_utf8_lossy(&output.stdout).to_string()
                            + &String::from_utf8_lossy(&output.stderr),
                        duration_millis: start.elapsed().as_millis(),
                        timed_out: false,
                    });
                }
                if start.elapsed() >= timeout {
                    let _ = child.kill();
                    let output = child.wait_with_output().map_err(|err| err.to_string())?;
                    return Ok(CommandResult {
                        exit_code: 124,
                        output: String::from_utf8_lossy(&output.stdout).to_string()
                            + &String::from_utf8_lossy(&output.stderr),
                        duration_millis: start.elapsed().as_millis(),
                        timed_out: true,
                    });
                }
                thread::sleep(Duration::from_millis(5));
            }
        }

        let output = child.wait_with_output().map_err(|err| err.to_string())?;
        Ok(CommandResult {
            exit_code: output.status.code().unwrap_or(1),
            output: String::from_utf8_lossy(&output.stdout).to_string()
                + &String::from_utf8_lossy(&output.stderr),
            duration_millis: start.elapsed().as_millis(),
            timed_out: false,
        })
    }
}

#[derive(Clone)]
pub struct ProcessTestCommandExecutor {
    command: Vec<String>,
    shell_command: Option<String>,
}

impl ProcessTestCommandExecutor {
    pub fn new(command: Vec<String>) -> Self {
        Self {
            command,
            shell_command: None,
        }
    }

    pub fn with_command(mut self, command: &str) -> Self {
        self.shell_command = Some(command.to_string());
        self
    }

    pub fn run_tests(&self, project_root: &Path, timeout_millis: u64) -> Result<TestRun, String> {
        let runner = ProcessCommandExecutor;
        let result = if let Some(command) = &self.shell_command {
            runner.run(&["sh", "-lc", command], project_root, timeout_millis)?
        } else if self.command.is_empty() {
            runner.run(&["cargo", "test"], project_root, timeout_millis)?
        } else {
            let refs = self.command.iter().map(String::as_str).collect::<Vec<_>>();
            runner.run(&refs, project_root, timeout_millis)?
        };

        Ok(TestRun {
            exit_code: result.exit_code,
            output: result.output,
            duration_millis: result.duration_millis,
            timed_out: result.timed_out,
        })
    }
}
