package mutate4go

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMutationCatalogDiscoversBooleanEqualityAndComparisonMutations(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "sample.go")
	writeFile(t, file, `package demo

func truthy() bool {
	return true
}

func same(left int, right int) bool {
	return left == right
}

func larger(left int, right int) bool {
	return left > right
}

func smaller(left int, right int) bool {
	return left <= right
}
`)

	sites, err := MutationCatalog{}.Discover([]string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDescriptions(t, sites, []string{
		"replace true with false",
		"replace == with !=",
		"replace > with >=",
		"replace <= with <",
	})
}

func TestMutationCatalogIgnoresOperatorsInsideStringsAndComments(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "literals.go")
	writeFile(t, file, `package demo

func text(left int, right int) string {
	_ = "true == false > <"
	// left == right > 0
	/* false != true <= >= */
	if left == right {
		return "same"
	}
	return "different"
}
`)

	sites, err := MutationCatalog{}.Discover([]string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDescriptions(t, sites, []string{"replace == with !="})
}

func TestMutationCatalogDiscoversArithmeticLogicalUnaryAndConstantMutations(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "expanded.go")
	writeFile(t, file, `package demo

func add(left int, right int) int {
	return left + right
}

func divide(left int, right int) int {
	return left / right
}

func both(left bool, right bool) bool {
	return left && right
}

func invert(value bool) bool {
	return !value
}

func negative(value int) int {
	return -value
}

func zero() int {
	return 0
}

func one() int {
	return 1
}
`)

	sites, err := MutationCatalog{}.Discover([]string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDescriptions(t, sites, []string{
		"replace + with -",
		"replace / with *",
		"replace && with ||",
		"replace ! with removed !",
		"replace - with removed -",
		"replace 0 with 1",
		"replace 1 with 0",
	})
	if sites[3].ReplacementText != "" || sites[4].ReplacementText != "" {
		t.Fatalf("expected unary removals to use empty replacement text: %#v", sites)
	}
}

func TestMutationCatalogAnalyzeReturnsScopesAndModuleHash(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "sample.go")
	writeFile(t, file, `package demo

func truthy() bool {
	return true
}

func same(left int, right int) bool {
	return left == right
}
`)

	analysis, err := MutationCatalog{}.Analyze(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis.ModuleHash == "" {
		t.Fatal("expected module hash")
	}
	if len(analysis.Scopes) == 0 {
		t.Fatal("expected scopes")
	}
	if len(analysis.Sites) != 2 {
		t.Fatalf("unexpected site count: %d", len(analysis.Sites))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func assertDescriptions(t *testing.T, sites []MutationSite, want []string) {
	t.Helper()
	if len(sites) != len(want) {
		t.Fatalf("unexpected site count %d, want %d: %#v", len(sites), len(want), sites)
	}
	for i := range want {
		if sites[i].Description != want[i] {
			t.Fatalf("site %d description = %q, want %q", i, sites[i].Description, want[i])
		}
	}
}
