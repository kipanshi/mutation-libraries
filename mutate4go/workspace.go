package mutate4go

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func prepareWorkerRoots(moduleRoot string, workers int) ([]string, func(), error) {
	runRoot := filepath.Join(moduleRoot, ".mutate", "workers", "run-"+intToString(int(time.Now().UnixNano())))
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		return nil, nil, err
	}
	roots := make([]string, 0, workers)
	for worker := 1; worker <= workers; worker++ {
		workerRoot := filepath.Join(runRoot, "worker-"+intToString(worker))
		if err := copyModuleTree(moduleRoot, workerRoot); err != nil {
			return nil, func() { _ = os.RemoveAll(runRoot) }, err
		}
		roots = append(roots, workerRoot)
	}
	return roots, func() { _ = os.RemoveAll(runRoot) }, nil
}

func copyModuleTree(sourceRoot string, destinationRoot string) error {
	return filepath.Walk(sourceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(sourceRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == ".mutate" || strings.HasPrefix(rel, ".mutate/") || rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		destination := filepath.Join(destinationRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(destination, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
