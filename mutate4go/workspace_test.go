package mutate4go

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWorkerRootsCreatesWorkerCopiesWithoutCopyingMutateOutput(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.18\n")
	writeFile(t, filepath.Join(root, "demo", "app.go"), "package demo\n")
	writeFile(t, filepath.Join(root, ".mutate", "workers", "old", "ignored.txt"), "ignored")

	workerRoots, cleanup, err := prepareWorkerRoots(root, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	workerRoot := workerRoots[0]
	if _, err := os.Stat(filepath.Join(workerRoot, "go.mod")); err != nil {
		t.Fatalf("expected go.mod in worker root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerRoot, "demo", "app.go")); err != nil {
		t.Fatalf("expected source file in worker root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workerRoot, ".mutate", "workers", "old", "ignored.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected mutate output to be excluded, got err=%v", err)
	}
}

func TestPrepareWorkerRootsCleanupRemovesRunDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.18\n")
	writeFile(t, filepath.Join(root, "demo", "app.go"), "package demo\n")

	workerRoots, cleanup, err := prepareWorkerRoots(root, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	runRoot := filepath.Dir(workerRoots[0])
	writeFile(t, filepath.Join(workerRoots[0], ".mutate", "tmp", "result.txt"), "done")

	cleanup()

	if _, err := os.Stat(runRoot); !os.IsNotExist(err) {
		t.Fatalf("expected run root removed, got err=%v", err)
	}
}
