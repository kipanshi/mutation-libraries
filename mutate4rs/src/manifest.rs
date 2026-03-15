use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
};

use crate::model::{ChangedScopes, DifferentialManifest, SourceAnalysis};

pub struct ManifestStore {
    workspace_root: PathBuf,
}

impl ManifestStore {
    pub fn new(workspace_root: PathBuf) -> Self {
        Self { workspace_root }
    }

    pub fn read(&self, file_arg: &str) -> Result<Option<DifferentialManifest>, String> {
        let path = self.manifest_path(file_arg);
        if !path.exists() {
            return Ok(None);
        }
        let content = fs::read_to_string(path).map_err(|err| err.to_string())?;
        serde_json::from_str(&content)
            .map(Some)
            .map_err(|err| err.to_string())
    }

    pub fn write(&self, file_arg: &str, analysis: &SourceAnalysis) -> Result<(), String> {
        let path = self.manifest_path(file_arg);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|err| err.to_string())?;
        }
        let manifest = DifferentialManifest {
            version: 1,
            module_hash: analysis.module_hash.clone(),
            scopes: analysis.scopes.clone(),
        };
        let content = serde_json::to_string_pretty(&manifest).map_err(|err| err.to_string())?;
        fs::write(path, content).map_err(|err| err.to_string())
    }

    pub fn changed_scopes(
        &self,
        file_arg: &str,
        analysis: &SourceAnalysis,
    ) -> Result<ChangedScopes, String> {
        let Some(manifest) = self.read(file_arg)? else {
            return Ok(ChangedScopes::default());
        };
        if manifest.module_hash == analysis.module_hash {
            return Ok(ChangedScopes {
                manifest_present: true,
                module_hash_changed: false,
                ..ChangedScopes::default()
            });
        }

        let previous = manifest
            .scopes
            .into_iter()
            .map(|scope| (scope.id, scope.semantic_hash))
            .collect::<BTreeMap<_, _>>();
        let mut changed = ChangedScopes {
            manifest_present: true,
            module_hash_changed: true,
            ..ChangedScopes::default()
        };
        for scope in &analysis.scopes {
            match previous.get(&scope.id) {
                None => {
                    changed.unregistered_scope_ids.insert(scope.id.clone());
                }
                Some(old_hash) if old_hash != &scope.semantic_hash => {
                    changed
                        .manifest_violation_scope_ids
                        .insert(scope.id.clone());
                }
                _ => {}
            }
        }
        Ok(changed)
    }

    fn manifest_path(&self, file_arg: &str) -> PathBuf {
        let relative = Path::new(file_arg);
        self.workspace_root
            .join(".mutate")
            .join("manifests")
            .join(relative.parent().unwrap_or_else(|| Path::new("")))
            .join(format!(
                "{}.json",
                relative
                    .file_name()
                    .and_then(|name| name.to_str())
                    .unwrap_or("unknown")
            ))
    }
}
