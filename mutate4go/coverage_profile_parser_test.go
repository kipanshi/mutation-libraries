package mutate4go

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCoverageProfileParsesCoveredLines(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "coverage.out")
	writeFile(t, profile, `mode: set
example.com/demo/demo/flag.go:3.21,3.36 1 1
example.com/demo/demo/flag.go:5.1,5.2 1 0
`)

	report, err := ParseCoverageProfile(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Covers("example.com/demo/demo/flag.go", 3) {
		t.Fatal("expected line 3 covered")
	}
	if report.Covers("example.com/demo/demo/flag.go", 5) {
		t.Fatal("expected line 5 uncovered")
	}
}

func TestParseCoverageProfileIgnoresMalformedLines(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "coverage.out")
	writeFile(t, profile, `mode: set
broken line
`)

	report, err := ParseCoverageProfile(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Covers("broken", 0) {
		t.Fatal("expected malformed line to be ignored")
	}
	if _, statErr := os.Stat(profile); statErr != nil {
		t.Fatalf("profile should still exist: %v", statErr)
	}
}
