package mutate4go

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveSourceFile(workspaceRoot string, target string) (string, error) {
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceRoot, target)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("mutate4go target must be a .go file")
	}
	if !strings.HasSuffix(path, ".go") {
		return "", fmt.Errorf("mutate4go target must be a .go file")
	}
	return filepath.Clean(path), nil
}

func findModuleRoot(sourceFile string, workspaceRoot string) string {
	current := filepath.Dir(sourceFile)
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		if current == filepath.Dir(current) {
			break
		}
		current = filepath.Dir(current)
	}
	return workspaceRoot
}

func modulePath(moduleRoot string) string {
	file, err := os.Open(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func manifestPath(moduleRoot string, sourceFile string) string {
	rel, err := filepath.Rel(moduleRoot, sourceFile)
	if err != nil {
		rel = filepath.Base(sourceFile)
	}
	return filepath.Join(moduleRoot, ".mutate", "manifests", filepath.FromSlash(filepath.ToSlash(rel))+".json")
}

func relativeSourcePath(moduleRoot string, sourceFile string) string {
	rel, err := filepath.Rel(moduleRoot, sourceFile)
	if err != nil {
		return filepath.ToSlash(filepath.Base(sourceFile))
	}
	return filepath.ToSlash(rel)
}

func coveragePathVariants(moduleRoot string, modPath string, sourceFile string) []string {
	rel := relativeSourcePath(moduleRoot, sourceFile)
	paths := []string{rel}
	if modPath != "" {
		paths = append(paths, filepath.ToSlash(modPath+"/"+rel))
	}
	return paths
}
