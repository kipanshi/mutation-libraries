package mutate4go

import (
	"path/filepath"
	"testing"
)

func TestFindModuleRootUsesWorkspaceRootWhenNoGoModExistsAboveTarget(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)

	moduleRoot := findModuleRoot(file, root)
	if moduleRoot != root {
		t.Fatalf("expected workspace root %q, got %q", root, moduleRoot)
	}
}

func TestFindModuleRootFindsNearestGoMod(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "tools", "mutate4go")
	writeFile(t, filepath.Join(moduleRoot, "go.mod"), "module example.com/tools/mutate4go\n\ngo 1.18\n")
	file := filepath.Join(moduleRoot, "demo", "sample.go")
	writeFile(t, file, `package demo

func truthy() bool {
	return true
}
`)

	resolved := findModuleRoot(file, root)
	if resolved != moduleRoot {
		t.Fatalf("expected module root %q, got %q", moduleRoot, resolved)
	}
}

func TestRelativeSourcePathKeepsRelativePathWhenFileIsOutsideSpecialTrees(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "demo", "sample.go")
	writeFile(t, file, "package demo\n")

	if got := relativeSourcePath(root, file); got != "demo/sample.go" {
		t.Fatalf("expected relative source path demo/sample.go, got %q", got)
	}
}

func TestCoveragePathVariantsIncludeRelativeAndModuleQualifiedPaths(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "demo", "sample.go")
	writeFile(t, file, "package demo\n")

	variants := coveragePathVariants(root, "example.com/demo", file)
	if len(variants) != 2 || variants[0] != "demo/sample.go" || variants[1] != "example.com/demo/demo/sample.go" {
		t.Fatalf("unexpected variants: %#v", variants)
	}
}
