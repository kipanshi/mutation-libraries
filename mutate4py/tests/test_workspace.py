from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.workspace import WorkerWorkspaces, prepare_worker_roots


class WorkerWorkspacesTest(TestCase):
    def test_prepare_worker_roots_copies_project_without_mutate_output(self) -> None:
        root = self._temp_dir()
        Path(root, "demo").mkdir(parents=True, exist_ok=True)
        Path(root, "demo", "sample.py").write_text("def value():\n    return True\n")
        Path(root, ".mutate", "workers", "old").mkdir(parents=True, exist_ok=True)
        Path(root, ".mutate", "workers", "old", "ignored.txt").write_text("ignored")

        workspaces = prepare_worker_roots(root, 2)
        self.addCleanup(workspaces.close)

        worker_root = workspaces.worker_roots[0]
        self.assertTrue(Path(worker_root, "demo", "sample.py").exists())
        self.assertFalse(
            Path(worker_root, ".mutate", "workers", "old", "ignored.txt").exists()
        )

    def test_close_removes_run_root(self) -> None:
        root = self._temp_dir()
        Path(root, "demo").mkdir(parents=True, exist_ok=True)
        Path(root, "demo", "sample.py").write_text("def value():\n    return True\n")

        workspaces = prepare_worker_roots(root, 1)
        run_root = Path(workspaces.run_root)
        Path(run_root, "worker-1", ".mutate", "tmp").mkdir(parents=True, exist_ok=True)
        Path(run_root, "worker-1", ".mutate", "tmp", "result.txt").write_text("done")

        workspaces.close()

        self.assertFalse(run_root.exists())

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()
