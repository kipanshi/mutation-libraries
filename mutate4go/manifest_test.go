package mutate4go

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestStoreWritesReadsAndTracksManifest(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	analysis, err := MutationCatalog{}.Analyze(file)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	store := ManifestStore{}
	if err := store.Write(root, file, analysis); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, present, err := store.Read(root, file)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !present {
		t.Fatal("expected manifest to be present")
	}
	if manifest.Version != 1 || manifest.ModuleHash == "" || len(manifest.Scopes) == 0 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(root, ".mutate", "manifests", "demo", "sample.go.json")); err != nil {
		t.Fatalf("expected manifest path to exist: %v", err)
	}
}

func TestManifestStoreChangedScopesReturnsNoChangesWhenModuleHashMatches(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	analysis, err := MutationCatalog{}.Analyze(file)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	store := ManifestStore{}
	if err := store.Write(root, file, analysis); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	changed, err := store.ChangedScopes(root, file, analysis)
	if err != nil {
		t.Fatalf("changed scopes: %v", err)
	}
	if !changed.ManifestPresent || changed.ModuleHashChanged || len(changed.AllScopeIDs()) != 0 {
		t.Fatalf("unexpected changed scopes: %#v", changed)
	}
}

func TestManifestStoreChangedScopesReportsUnregisteredAndManifestViolations(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	store := ManifestStore{}
	baseline, err := MutationCatalog{}.Analyze(file)
	if err != nil {
		t.Fatalf("analyze baseline: %v", err)
	}
	if err := store.Write(root, file, baseline); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	writeFile(t, file, `package demo

func truthy() bool {
	return false
}

func same(left int, right int) bool {
	return left == right
}

func brandNew() bool {
	return true
}
`)
	current, err := MutationCatalog{}.Analyze(file)
	if err != nil {
		t.Fatalf("analyze current: %v", err)
	}

	changed, err := store.ChangedScopes(root, file, current)
	if err != nil {
		t.Fatalf("changed scopes: %v", err)
	}
	if !changed.ManifestPresent || !changed.ModuleHashChanged {
		t.Fatalf("expected manifest present and changed hash: %#v", changed)
	}
	if len(changed.UnregisteredScopeIDs) != 1 || len(changed.ManifestViolationScopes) != 1 {
		t.Fatalf("unexpected changed scope buckets: %#v", changed)
	}
}
