from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import shutil
import time


@dataclass
class WorkerWorkspaces:
    run_root: str
    worker_roots: list[str]

    def close(self) -> None:
        run_root = Path(self.run_root)
        if run_root.exists():
            shutil.rmtree(run_root)


def prepare_worker_roots(workspace_root: str, worker_count: int) -> WorkerWorkspaces:
    run_root = Path(workspace_root, ".mutate", "workers", f"run-{time.time_ns()}")
    run_root.mkdir(parents=True, exist_ok=True)
    worker_roots: list[str] = []
    for index in range(1, worker_count + 1):
        worker_root = run_root / f"worker-{index}"
        _copy_tree(Path(workspace_root), worker_root)
        worker_roots.append(str(worker_root))
    return WorkerWorkspaces(str(run_root), worker_roots)


def _copy_tree(source: Path, destination: Path) -> None:
    for path in source.rglob("*"):
        relative = path.relative_to(source)
        if _should_skip(relative):
            continue
        target = destination / relative
        if path.is_dir():
            target.mkdir(parents=True, exist_ok=True)
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(path, target)


def _should_skip(relative: Path) -> bool:
    parts = relative.parts
    return ".mutate" in parts or ".git" in parts
