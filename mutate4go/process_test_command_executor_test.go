package mutate4go

import "testing"

func TestProcessTestCommandExecutorCapturesSuccessfulTestRunOutput(t *testing.T) {
	result, err := NewProcessTestCommandExecutor([]string{"sh", "-c", "printf ok"}).RunTests(t.TempDir(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 || result.Output != "ok" || result.TimedOut {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProcessTestCommandExecutorReturnsTimeoutExitCodeWhenTestRunTakesTooLong(t *testing.T) {
	result, err := NewProcessTestCommandExecutor([]string{"sh", "-c", "sleep 1"}).RunTests(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 124 || !result.TimedOut {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProcessTestCommandExecutorStartsShellCommandOverrideInTargetDirectory(t *testing.T) {
	result, err := NewProcessTestCommandExecutor(nil).WithCommand("printf ok").RunTests(t.TempDir(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 || result.Output != "ok" || result.TimedOut {
		t.Fatalf("unexpected result: %#v", result)
	}
}
