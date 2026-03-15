use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use mutate4rs::{ManifestStore, MutationCatalog};

#[test]
fn writes_reads_and_tracks_manifest() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let analysis = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();

    let store = ManifestStore::new(root.clone());
    store.write(&file_arg, &analysis).unwrap();

    let manifest = store.read(&file_arg).unwrap().unwrap();
    assert_eq!(1, manifest.version);
    assert!(!manifest.module_hash.is_empty());
    assert!(!manifest.scopes.is_empty());
}

#[test]
fn reports_no_changes_when_manifest_matches() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let analysis = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();
    let store = ManifestStore::new(root.clone());
    store.write(&file_arg, &analysis).unwrap();

    let changed = store.changed_scopes(&file_arg, &analysis).unwrap();

    assert!(changed.manifest_present);
    assert!(!changed.module_hash_changed);
    assert!(changed.all_scope_ids().is_empty());
}

#[test]
fn reports_unregistered_and_manifest_violations() {
    let root = temp_dir();
    let file_arg = write_source_file(&root, "pub fn truthy() -> bool { true }\n");
    let store = ManifestStore::new(root.clone());
    let baseline = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();
    store.write(&file_arg, &baseline).unwrap();

    fs::write(
        root.join(&file_arg),
        "pub fn truthy() -> bool { false }\npub fn brand_new() -> bool { true }\n",
    )
    .unwrap();
    let current = MutationCatalog.analyze(&root.join(&file_arg)).unwrap();

    let changed = store.changed_scopes(&file_arg, &current).unwrap();
    assert!(changed.manifest_present);
    assert!(changed.module_hash_changed);
    assert_eq!(1, changed.unregistered_scope_ids.len());
    assert_eq!(1, changed.manifest_violation_scope_ids.len());
}

fn temp_dir() -> PathBuf {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("mutate4rs-manifest-{unique}"));
    fs::create_dir_all(&dir).unwrap();
    dir
}

fn write_source_file(root: &PathBuf, content: &str) -> String {
    let file_arg = "src/lib.rs".to_string();
    let path = root.join(&file_arg);
    fs::create_dir_all(path.parent().unwrap()).unwrap();
    fs::write(path, content).unwrap();
    file_arg
}
