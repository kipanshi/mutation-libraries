use std::collections::{BTreeMap, BTreeSet};

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CliMode {
    ExplicitFiles,
    Help,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CliArguments {
    pub mode: CliMode,
    pub file_args: Vec<String>,
    pub lines: BTreeSet<usize>,
    pub scan: bool,
    pub update_manifest: bool,
    pub reuse_coverage: bool,
    pub since_last_run: bool,
    pub mutate_all: bool,
    pub timeout_factor: usize,
    pub mutation_warning: usize,
    pub max_workers: usize,
    pub test_command: Option<String>,
    pub verbose: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MutationSite {
    pub file: String,
    pub line: usize,
    pub start: usize,
    pub end: usize,
    pub original_text: String,
    pub replacement_text: String,
    pub description: String,
    pub scope_id: String,
    pub scope_kind: String,
    pub scope_start_line: usize,
    pub scope_end_line: usize,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MutationScope {
    pub id: String,
    pub kind: String,
    pub start_line: usize,
    pub end_line: usize,
    pub semantic_hash: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SourceAnalysis {
    pub source: String,
    pub sites: Vec<MutationSite>,
    pub scopes: Vec<MutationScope>,
    pub module_hash: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DifferentialManifest {
    pub version: usize,
    pub module_hash: String,
    pub scopes: Vec<MutationScope>,
}

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ChangedScopes {
    pub manifest_present: bool,
    pub module_hash_changed: bool,
    pub unregistered_scope_ids: BTreeSet<String>,
    pub manifest_violation_scope_ids: BTreeSet<String>,
}

impl ChangedScopes {
    pub fn all_scope_ids(&self) -> BTreeSet<String> {
        self.unregistered_scope_ids
            .union(&self.manifest_violation_scope_ids)
            .cloned()
            .collect()
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct CoverageReport {
    pub covered: BTreeMap<String, BTreeSet<usize>>,
}

impl CoverageReport {
    pub fn covers(&self, path: &str, line: usize) -> bool {
        self.covered
            .get(path)
            .is_some_and(|lines| lines.contains(&line))
    }
}
