from __future__ import annotations

import json
from pathlib import Path

from mutate4py.model import (
    ChangedScopes,
    DifferentialManifest,
    MutationScope,
    SourceAnalysis,
)


class ManifestStore:
    def __init__(self, workspace_root: str) -> None:
        self.workspace_root = Path(workspace_root)

    def read(self, file_path: str) -> DifferentialManifest | None:
        path = self._manifest_path(file_path)
        if not path.exists():
            return None
        payload = json.loads(path.read_text(encoding="utf-8"))
        return DifferentialManifest(
            version=payload["version"],
            module_hash=payload["module_hash"],
            scopes=[MutationScope(**scope) for scope in payload["scopes"]],
        )

    def write(self, file_path: str, analysis: SourceAnalysis) -> None:
        path = self._manifest_path(file_path)
        path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "version": 1,
            "module_hash": analysis.module_hash,
            "scopes": [scope.__dict__ for scope in analysis.scopes],
        }
        path.write_text(json.dumps(payload, indent=2), encoding="utf-8")

    def changed_scopes(self, file_path: str, analysis: SourceAnalysis) -> ChangedScopes:
        manifest = self.read(file_path)
        if manifest is None:
            return ChangedScopes(manifest_present=False, module_hash_changed=False)
        if manifest.module_hash == analysis.module_hash:
            return ChangedScopes(manifest_present=True, module_hash_changed=False)

        previous = {scope.id: scope.semantic_hash for scope in manifest.scopes}
        unregistered: set[str] = set()
        violations: set[str] = set()
        for scope in analysis.scopes:
            previous_hash = previous.get(scope.id)
            if previous_hash is None:
                unregistered.add(scope.id)
            elif previous_hash != scope.semantic_hash:
                violations.add(scope.id)
        return ChangedScopes(
            manifest_present=True,
            module_hash_changed=True,
            unregistered_scope_ids=unregistered,
            manifest_violation_scope_ids=violations,
        )

    def _manifest_path(self, file_path: str) -> Path:
        relative = Path(file_path)
        return (
            self.workspace_root
            / ".mutate"
            / "manifests"
            / relative.parent
            / f"{relative.name}.json"
        )
