package mutate4go

import (
	"context"
	"os/exec"
	"time"
)

func (e ProcessCommandExecutor) Run(command []string, dir string, timeoutMillis int64) (CommandResult, error) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if timeoutMillis > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutMillis)*time.Millisecond)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	started := time.Now()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	duration := time.Since(started).Milliseconds()
	if ctx.Err() == context.DeadlineExceeded {
		return CommandResult{ExitCode: 124, Output: string(output), DurationMillis: duration, TimedOut: true}, nil
	}
	if err == nil {
		return CommandResult{ExitCode: 0, Output: string(output), DurationMillis: duration}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return CommandResult{ExitCode: exitErr.ExitCode(), Output: string(output), DurationMillis: duration}, nil
	}
	return CommandResult{}, err
}

func NewProcessTestCommandExecutor(command []string) ProcessTestCommandExecutor {
	return ProcessTestCommandExecutor{command: command, runner: ProcessCommandExecutor{}}
}

func (e ProcessTestCommandExecutor) WithCommand(command string) ProcessTestCommandExecutor {
	e.shellCommand = command
	return e
}

func (e ProcessTestCommandExecutor) RunTests(projectRoot string, timeoutMillis int64) (TestRun, error) {
	command := e.command
	if len(command) == 0 {
		command = []string{"go", "test", "./..."}
	}
	if e.shellCommand != "" {
		command = []string{"sh", "-lc", e.shellCommand}
	}
	result, err := e.runner.Run(command, projectRoot, timeoutMillis)
	if err != nil {
		return TestRun{}, err
	}
	return TestRun{
		ExitCode:       result.ExitCode,
		Output:         result.Output,
		DurationMillis: result.DurationMillis,
		TimedOut:       result.TimedOut,
	}, nil
}
