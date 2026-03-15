use std::{
    fs,
    path::{Path, PathBuf},
    time::{SystemTime, UNIX_EPOCH},
};

pub struct WorkerWorkspaces {
    pub run_root: String,
    pub worker_roots: Vec<String>,
}

impl WorkerWorkspaces {
    pub fn close(&self) -> Result<(), String> {
        let run_root = Path::new(&self.run_root);
        if run_root.exists() {
            fs::remove_dir_all(run_root).map_err(|err| err.to_string())?;
        }
        Ok(())
    }
}

pub fn prepare_worker_roots(
    workspace_root: &Path,
    worker_count: usize,
) -> Result<WorkerWorkspaces, String> {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|err| err.to_string())?
        .as_nanos();
    let run_root = workspace_root
        .join(".mutate")
        .join("workers")
        .join(format!("run-{unique}"));
    fs::create_dir_all(&run_root).map_err(|err| err.to_string())?;
    let mut worker_roots = Vec::new();
    for index in 1..=worker_count {
        let worker_root = run_root.join(format!("worker-{index}"));
        copy_tree(workspace_root, &worker_root)?;
        worker_roots.push(worker_root.to_string_lossy().to_string());
    }
    Ok(WorkerWorkspaces {
        run_root: run_root.to_string_lossy().to_string(),
        worker_roots,
    })
}

fn copy_tree(source: &Path, destination: &Path) -> Result<(), String> {
    for entry in walkdir(source)? {
        let relative = entry.strip_prefix(source).map_err(|err| err.to_string())?;
        if should_skip(relative) {
            continue;
        }
        let target = destination.join(relative);
        if entry.is_dir() {
            fs::create_dir_all(&target).map_err(|err| err.to_string())?;
        } else {
            if let Some(parent) = target.parent() {
                fs::create_dir_all(parent).map_err(|err| err.to_string())?;
            }
            fs::copy(&entry, &target).map_err(|err| err.to_string())?;
        }
    }
    Ok(())
}

fn walkdir(root: &Path) -> Result<Vec<PathBuf>, String> {
    let mut paths = vec![root.to_path_buf()];
    let mut index = 0;
    while index < paths.len() {
        let path = paths[index].clone();
        if path.is_dir() {
            for entry in fs::read_dir(&path).map_err(|err| err.to_string())? {
                let entry = entry.map_err(|err| err.to_string())?;
                paths.push(entry.path());
            }
        }
        index += 1;
    }
    Ok(paths)
}

fn should_skip(relative: &Path) -> bool {
    relative.components().any(|component| {
        let name = component.as_os_str().to_string_lossy();
        name == ".mutate" || name == ".git"
    })
}
