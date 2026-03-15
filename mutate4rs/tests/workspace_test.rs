use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use mutate4rs::prepare_worker_roots;

#[test]
fn prepare_worker_roots_copies_project_without_mutate_output() {
    let root = temp_dir();
    fs::create_dir_all(root.join("src")).unwrap();
    fs::write(root.join("src/lib.rs"), "pub fn value() -> bool { true }\n").unwrap();
    fs::create_dir_all(root.join(".mutate/workers/old")).unwrap();
    fs::write(root.join(".mutate/workers/old/ignored.txt"), "ignored").unwrap();

    let workspaces = prepare_worker_roots(&root, 2).unwrap();

    let worker_root = PathBuf::from(&workspaces.worker_roots[0]);
    assert!(worker_root.join("src/lib.rs").exists());
    assert!(!worker_root.join(".mutate/workers/old/ignored.txt").exists());
    workspaces.close().unwrap();
}

#[test]
fn close_removes_run_root() {
    let root = temp_dir();
    fs::create_dir_all(root.join("src")).unwrap();
    fs::write(root.join("src/lib.rs"), "pub fn value() -> bool { true }\n").unwrap();

    let workspaces = prepare_worker_roots(&root, 1).unwrap();
    let run_root = PathBuf::from(&workspaces.run_root);
    fs::create_dir_all(run_root.join("worker-1/.mutate/tmp")).unwrap();
    fs::write(run_root.join("worker-1/.mutate/tmp/result.txt"), "done").unwrap();

    workspaces.close().unwrap();
    assert!(!run_root.exists());
}

fn temp_dir() -> PathBuf {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("mutate4rs-workspace-{unique}"));
    fs::create_dir_all(&dir).unwrap();
    dir
}
