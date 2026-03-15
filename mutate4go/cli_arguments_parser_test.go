package mutate4go

import (
	"runtime"
	"testing"
)

func TestParseArgsRejectsMissingFileArgument(t *testing.T) {
	_, err := ParseArgs([]string{})
	if err == nil || err.Error() != "mutate4go requires exactly one Go file" {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestParseArgsParsesSingleExplicitFileArgument(t *testing.T) {
	parsed, err := ParseArgs([]string{"demo/flag.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Mode != ModeExplicitFiles {
		t.Fatalf("expected explicit mode, got %q", parsed.Mode)
	}
	if len(parsed.FileArgs) != 1 || parsed.FileArgs[0] != "demo/flag.go" {
		t.Fatalf("unexpected file args: %#v", parsed.FileArgs)
	}
	if len(parsed.Lines) != 0 || parsed.Scan || parsed.UpdateManifest || parsed.ReuseCoverage || parsed.SinceLastRun || parsed.MutateAll || parsed.Verbose {
		t.Fatalf("unexpected parsed flags: %#v", parsed)
	}
	if parsed.TimeoutFactor != 10 || parsed.MutationWarning != 50 || parsed.MaxWorkers != max(1, runtime.NumCPU()/2) {
		t.Fatalf("unexpected defaults: %#v", parsed)
	}
}

func TestParseArgsParsesLineFilterAndTimeoutFactor(t *testing.T) {
	parsed, err := ParseArgs([]string{"demo/flag.go", "--lines", "12,18", "--timeout-factor", "15"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := parsed.Lines[12]; !ok {
		t.Fatalf("missing line 12: %#v", parsed.Lines)
	}
	if _, ok := parsed.Lines[18]; !ok {
		t.Fatalf("missing line 18: %#v", parsed.Lines)
	}
	if parsed.TimeoutFactor != 15 {
		t.Fatalf("unexpected timeout factor: %d", parsed.TimeoutFactor)
	}
}

func TestParseArgsParsesDifferentialFlagsAndTestCommand(t *testing.T) {
	parsed, err := ParseArgs([]string{"demo/flag.go", "--since-last-run", "--mutation-warning", "75", "--test-command", "go test ./..."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.SinceLastRun || parsed.MutateAll || parsed.MutationWarning != 75 || parsed.TestCommand != "go test ./..." {
		t.Fatalf("unexpected parse result: %#v", parsed)
	}
}

func TestParseArgsParsesHelpMode(t *testing.T) {
	parsed, err := ParseArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Mode != ModeHelp {
		t.Fatalf("expected help mode, got %#v", parsed)
	}
}

func TestParseArgsRejectsNonGoTarget(t *testing.T) {
	_, err := ParseArgs([]string{"bogus"})
	if err == nil || err.Error() != "mutate4go target must be a .go file" {
		t.Fatalf("expected non-go target error, got %v", err)
	}
}

func TestParseArgsRejectsUnknownOption(t *testing.T) {
	_, err := ParseArgs([]string{"demo/flag.go", "--bogus"})
	if err == nil || err.Error() != "Unknown option: --bogus" {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

func TestParseArgsRejectsLinesCombinedWithSinceLastRun(t *testing.T) {
	_, err := ParseArgs([]string{"demo/flag.go", "--lines", "5", "--since-last-run"})
	if err == nil || err.Error() != "--lines may not be combined with --since-last-run" {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestParseArgsRejectsScanCombinedWithUpdateManifest(t *testing.T) {
	_, err := ParseArgs([]string{"demo/flag.go", "--scan", "--update-manifest"})
	if err == nil || err.Error() != "--scan may not be combined with --update-manifest" {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestParseArgsRejectsBlankTestCommand(t *testing.T) {
	_, err := ParseArgs([]string{"demo/flag.go", "--test-command", "   "})
	if err == nil || err.Error() != "--test-command must not be blank" {
		t.Fatalf("expected blank command error, got %v", err)
	}
}

func max(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
