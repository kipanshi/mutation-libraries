package mutate4go

import "testing"

func TestProcessCommandExecutorCapturesSuccessfulCommandOutput(t *testing.T) {
	result, err := ProcessCommandExecutor{}.Run([]string{"sh", "-c", "printf 'ok'"}, t.TempDir(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 || result.Output != "ok" || result.TimedOut {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProcessCommandExecutorReturnsTimeoutExitCodeWhenCommandTakesTooLong(t *testing.T) {
	result, err := ProcessCommandExecutor{}.Run([]string{"sh", "-c", "sleep 1"}, t.TempDir(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 124 || !result.TimedOut {
		t.Fatalf("unexpected result: %#v", result)
	}
}
