from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.analysis import MutationCatalog
from mutate4py.manifest import ManifestStore


class ManifestStoreTest(TestCase):
    def test_writes_reads_and_tracks_manifest(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root, "return True\n")
        analysis = MutationCatalog().analyze(str(Path(root, file_path)))

        store = ManifestStore(root)
        store.write(file_path, analysis)

        manifest = store.read(file_path)
        self.assertIsNotNone(manifest)
        assert manifest is not None
        self.assertEqual(1, manifest.version)
        self.assertTrue(manifest.module_hash)
        self.assertTrue(manifest.scopes)

    def test_reports_no_changes_when_manifest_matches(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root, "return True\n")
        analysis = MutationCatalog().analyze(str(Path(root, file_path)))
        store = ManifestStore(root)
        store.write(file_path, analysis)

        changed = store.changed_scopes(file_path, analysis)

        self.assertTrue(changed.manifest_present)
        self.assertFalse(changed.module_hash_changed)
        self.assertEqual(set(), changed.all_scope_ids())

    def test_reports_unregistered_and_manifest_violations(self) -> None:
        root = self._temp_dir()
        file_path = self._write_source_file(root, "return True\n")
        store = ManifestStore(root)
        baseline = MutationCatalog().analyze(str(Path(root, file_path)))
        store.write(file_path, baseline)

        Path(root, file_path).write_text(
            "def truthy():\n    return False\n\ndef brand_new():\n    return True\n",
            encoding="utf-8",
        )
        current = MutationCatalog().analyze(str(Path(root, file_path)))

        changed = store.changed_scopes(file_path, current)

        self.assertTrue(changed.manifest_present)
        self.assertTrue(changed.module_hash_changed)
        self.assertEqual(1, len(changed.unregistered_scope_ids))
        self.assertEqual(1, len(changed.manifest_violation_scope_ids))

    def _write_source_file(self, root: str, body: str) -> str:
        path = Path(root, "demo", "sample.py")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(f"def truthy():\n    {body}", encoding="utf-8")
        return "demo/sample.py"

    def _temp_dir(self) -> str:
        import tempfile

        return tempfile.mkdtemp()
