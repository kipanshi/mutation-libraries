package mutate4go

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPrintsHelp(t *testing.T) {
	var out bytes.Buffer
	exit := Run([]string{"--help"}, t.TempDir(), &out, &bytes.Buffer{})
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("mutate4go <file.go>")) {
		t.Fatalf("unexpected result: exit=%d out=%q", exit, out.String())
	}
}

func TestRunRejectsNonGoTargets(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer
	exit := Run([]string{"bogus"}, t.TempDir(), &out, &err)
	if exit != 1 || !bytes.Contains(err.Bytes(), []byte("target must be a .go file")) || !bytes.Contains(out.Bytes(), []byte("mutate4go <file.go>")) {
		t.Fatalf("unexpected result: exit=%d out=%q err=%q", exit, out.String(), err.String())
	}
}

func TestRunMutatesARealGoProject(t *testing.T) {
	root := t.TempDir()
	writePassingGoProject(t, root)
	var out bytes.Buffer
	var err bytes.Buffer

	exit := Run([]string{"demo/flag.go"}, root, &out, &err)
	if exit != 0 {
		t.Fatalf("unexpected exit: %d out=%q err=%q", exit, out.String(), err.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("KILLED demo/flag.go:4 replace true with false")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Summary: 1 killed, 0 survived, 1 total.")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunFailsFastWhenBaselineProjectTestsAreRed(t *testing.T) {
	root := t.TempDir()
	writeFailingGoProject(t, root)
	var err bytes.Buffer

	exit := Run([]string{"demo/flag.go"}, root, &bytes.Buffer{}, &err)
	if exit != 2 || !bytes.Contains(err.Bytes(), []byte("Baseline tests failed.")) {
		t.Fatalf("unexpected result: exit=%d err=%q", exit, err.String())
	}
}

func TestRunUsesCustomTestCommandAndTreatsSitesAsCovered(t *testing.T) {
	root := t.TempDir()
	writeTwoMutationGoProject(t, root)
	var out bytes.Buffer

	exit := Run([]string{"demo/pair.go", "--test-command", "go test ./..."}, root, &out, &bytes.Buffer{})
	if exit != 0 {
		t.Fatalf("unexpected exit: %d out=%q", exit, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Summary: 2 killed, 0 survived, 2 total.")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunAcceptsMaxWorkersDuringMutationRun(t *testing.T) {
	root := t.TempDir()
	writeTwoMutationGoProject(t, root)
	var out bytes.Buffer

	exit := Run([]string{"demo/pair.go", "--max-workers", "2"}, root, &out, &bytes.Buffer{})
	if exit != 0 {
		t.Fatalf("unexpected exit: %d out=%q", exit, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Summary: 2 killed, 0 survived, 2 total.")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunReportsUncoveredSitesFromCoverageAndSkipsThem(t *testing.T) {
	root := t.TempDir()
	writeUncoveredGoProject(t, root)
	var out bytes.Buffer

	exit := Run([]string{"demo/covered.go"}, root, &out, &bytes.Buffer{})
	if exit != 0 {
		t.Fatalf("unexpected exit: %d out=%q", exit, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("UNCOVERED demo/covered.go:8 replace false with true")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Coverage: 1 uncovered sites skipped.")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunUpdatesManifestWithoutRunningProjectTests(t *testing.T) {
	root := t.TempDir()
	writeFailingGoProject(t, root)
	var out bytes.Buffer
	var err bytes.Buffer

	exit := Run([]string{"demo/flag.go", "--update-manifest"}, root, &out, &err)
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("Updated manifest for demo/flag.go")) || err.Len() != 0 {
		t.Fatalf("unexpected result: exit=%d out=%q err=%q", exit, out.String(), err.String())
	}
	if _, statErr := os.Stat(filepath.Join(root, ".mutate", "manifests", "demo", "flag.go.json")); statErr != nil {
		t.Fatalf("expected manifest: %v", statErr)
	}
}

func writePassingGoProject(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.18\n")
	writeFile(t, filepath.Join(root, "demo", "flag.go"), `package demo

func Enabled() bool {
	return true
}
`)
	writeFile(t, filepath.Join(root, "demo", "flag_test.go"), `package demo

import "testing"

func TestEnabled(t *testing.T) {
	if !Enabled() {
		t.Fatal("expected enabled")
	}
}
`)
}

func writeFailingGoProject(t *testing.T, root string) {
	t.Helper()
	writePassingGoProject(t, root)
	writeFile(t, filepath.Join(root, "demo", "flag_test.go"), `package demo

import "testing"

func TestEnabled(t *testing.T) {
	if Enabled() {
		t.Fatal("expected disabled")
	}
}
`)
}

func writeUncoveredGoProject(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.18\n")
	writeFile(t, filepath.Join(root, "demo", "covered.go"), `package demo

func Exercised() bool {
	return true
}

func NotExercised() bool {
	return false
}
`)
	writeFile(t, filepath.Join(root, "demo", "covered_test.go"), `package demo

import "testing"

func TestExercised(t *testing.T) {
	if !Exercised() {
		t.Fatal("expected true")
	}
}
`)
}

func writeTwoMutationGoProject(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.18\n")
	writeFile(t, filepath.Join(root, "demo", "pair.go"), `package demo

func First() bool {
	return true
}

func Second() bool {
	return false
}
`)
	writeFile(t, filepath.Join(root, "demo", "pair_test.go"), `package demo

import "testing"

func TestPair(t *testing.T) {
	if !First() {
		t.Fatal("expected first true")
	}
		if Second() {
			t.Fatal("expected second false")
		}
	}
`)
}
