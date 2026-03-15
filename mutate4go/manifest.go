package mutate4go

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (m ManifestStore) Read(moduleRoot string, sourceFile string) (DifferentialManifest, bool, error) {
	path := manifestPath(moduleRoot, sourceFile)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DifferentialManifest{}, false, nil
		}
		return DifferentialManifest{}, false, err
	}
	var manifest DifferentialManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return DifferentialManifest{}, false, err
	}
	return manifest, true, nil
}

func (m ManifestStore) Write(moduleRoot string, sourceFile string, analysis SourceAnalysis) error {
	manifest := DifferentialManifest{Version: 1, ModuleHash: analysis.ModuleHash, Scopes: analysis.Scopes}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	path := manifestPath(moduleRoot, sourceFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func (m ManifestStore) ChangedScopes(moduleRoot string, sourceFile string, analysis SourceAnalysis) (ChangedScopes, error) {
	manifest, present, err := m.Read(moduleRoot, sourceFile)
	if err != nil {
		return ChangedScopes{}, err
	}
	if !present {
		return ChangedScopes{UnregisteredScopeIDs: map[string]struct{}{}, ManifestViolationScopes: map[string]struct{}{}}, nil
	}
	if manifest.ModuleHash == analysis.ModuleHash {
		return ChangedScopes{ManifestPresent: true, ModuleHashChanged: false, UnregisteredScopeIDs: map[string]struct{}{}, ManifestViolationScopes: map[string]struct{}{}}, nil
	}
	previous := map[string]string{}
	for _, scope := range manifest.Scopes {
		previous[scope.ID] = scope.SemanticHash
	}
	unregistered := map[string]struct{}{}
	violations := map[string]struct{}{}
	for _, scope := range analysis.Scopes {
		prevHash, ok := previous[scope.ID]
		if !ok {
			unregistered[scope.ID] = struct{}{}
			continue
		}
		if prevHash != scope.SemanticHash {
			violations[scope.ID] = struct{}{}
		}
	}
	return ChangedScopes{ManifestPresent: true, ModuleHashChanged: true, UnregisteredScopeIDs: unregistered, ManifestViolationScopes: violations}, nil
}
