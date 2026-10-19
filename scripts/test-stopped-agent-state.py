#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Real filesystem tests for private native-probe checkpoint handling."""
import importlib
import os
import errno
import shutil
from pathlib import Path
import tempfile
import unittest

try:
    backup = importlib.import_module('stopped_agent_state')
except ModuleNotFoundError:
    backup = None


class BackupTests(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(backup, 'complete stopped-state capture/restore helper missing')
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.state = self.root / 'state'; self.state.mkdir(mode=0o700)
        for name in ['owner.json', 'desired.json', 'owner.json.lock', 'desired.json.lock']:
            (self.state / name).write_bytes(('private-' + name).encode())
            (self.state / name).chmod(0o600)
        self.token = self.root / 'token'; self.token.write_text('private-token'); self.token.chmod(0o600)
        self.unit = self.root / 'unit'; self.unit.write_text('exact-rendered-unit')
        self.archive = self.root / 'backup'

    def capture(self, stopped=lambda: True):
        return backup.capture(self.state, self.token, self.unit, self.archive, stopped)

    def quarantine(self):
        self.state.rename(self.root / 'state-original')
        self.token.rename(self.root / 'token-original')
        self.unit.rename(self.root / 'unit-original')

    def test_roundtrip_copies_all_six_files_at_original_paths(self):
        self.capture(); self.quarantine()
        proof = backup.restore(self.state, self.token, self.unit, self.archive, lambda: True)
        self.assertEqual(proof, {'files': 6, 'byteIdentical': True, 'independentCopies': True,
                                 'private': True, 'originalPaths': True, 'stopped': True})
        for name in ['owner.json', 'desired.json', 'owner.json.lock', 'desired.json.lock']:
            self.assertEqual((self.state / name).read_bytes(), ('private-' + name).encode())
            self.assertNotEqual((self.state / name).stat().st_ino,
                                (self.archive / 'state' / name).stat().st_ino)
            self.assertEqual((self.state / name).stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.token.read_text(), 'private-token')
        self.assertEqual(self.unit.read_text(), 'exact-rendered-unit')

    def test_running_capture_has_no_side_effect(self):
        with self.assertRaises(ValueError): self.capture(lambda: False)
        self.assertFalse(self.archive.exists())
        self.assertEqual(self.token.read_text(), 'private-token')

    def test_running_restore_leaves_originals_quarantined(self):
        self.capture(); self.quarantine()
        with self.assertRaises(ValueError):
            backup.restore(self.state, self.token, self.unit, self.archive, lambda: False)
        self.assertFalse(self.state.exists()); self.assertFalse(self.token.exists())
        self.assertEqual((self.root / 'token-original').read_text(), 'private-token')

    def test_source_links_broad_modes_and_incomplete_state_rejected(self):
        for kind in ['symlink', 'hardlink', 'mode', 'missing', 'extra']:
            with self.subTest(kind=kind):
                target = self.state / 'owner.json'; original = target.read_bytes()
                if kind == 'symlink': target.unlink(); target.symlink_to(self.token)
                if kind == 'hardlink': target.unlink(); os.link(self.token, target)
                if kind == 'mode': target.chmod(0o644)
                if kind == 'missing': target.unlink()
                if kind == 'extra': (self.state / 'unknown').touch(mode=0o600)
                try:
                    with self.assertRaises(ValueError): self.capture()
                    self.assertFalse(self.archive.exists())
                finally:
                    target.unlink(missing_ok=True); target.write_bytes(original); target.chmod(0o600)
                    (self.state / 'unknown').unlink(missing_ok=True)

    def test_tampered_backup_refused_before_any_restore(self):
        self.capture(); self.quarantine()
        (self.archive / 'state' / 'desired.json').write_text('forged-regrant')
        with self.assertRaises(ValueError):
            backup.restore(self.state, self.token, self.unit, self.archive, lambda: True)
        self.assertFalse(self.state.exists()); self.assertFalse(self.token.exists())

    def test_changed_paths_refused(self):
        self.capture(); self.quarantine()
        other = self.root / 'different-state'
        with self.assertRaises(ValueError):
            backup.restore(other, self.token, self.unit, self.archive, lambda: True)
        self.assertFalse(other.exists())

    def test_live_destination_never_overwritten(self):
        self.capture()
        with self.assertRaises(ValueError):
            backup.restore(self.state, self.token, self.unit, self.archive, lambda: True)
        self.assertEqual(self.token.read_text(), 'private-token')

    def test_restarted_during_capture_rejected(self):
        calls = []
        def stopped():
            calls.append(1); return len(calls) == 1
        with self.assertRaises(ValueError): self.capture(stopped)

    def test_state_quarantine_is_on_original_filesystem(self):
        # A genuine Linux mount boundary, not a mocked rename.
        with tempfile.TemporaryDirectory(dir='/dev/shm') as other:
            remote = Path(other)
            self.assertNotEqual(remote.stat().st_dev, self.root.stat().st_dev)
            with self.assertRaises(OSError) as error:
                self.state.rename(remote / 'wrong-quarantine')
            self.assertEqual(error.exception.errno, errno.EXDEV)
            self.assertTrue(callable(getattr(backup, 'quarantine_state', None)),
                            'same-filesystem stopped-state quarantine missing')
            saved = backup.quarantine_state(self.state)
            self.assertEqual(saved.parent, self.state.parent)
            self.assertEqual(saved.stat().st_dev, self.root.stat().st_dev)
            self.assertEqual((saved / 'owner.json').read_bytes(), b'private-owner.json')

    def test_failed_recovery_keeps_backup_originals_and_partial_restore(self):
        self.capture(); self.quarantine()
        self.state.mkdir(mode=0o700)
        (self.state / 'partial').write_text('incomplete')
        runtime = self.root / 'unit-runtime'; runtime.touch()
        self.assertTrue(callable(getattr(backup, 'finish_cleanup', None)),
                        'failure-aware checkpoint lifecycle missing')
        backup.finish_cleanup(False, runtime.unlink, lambda: shutil.rmtree(self.archive))
        self.assertFalse(runtime.exists())
        self.assertTrue(self.archive.exists())
        self.assertTrue((self.root / 'state-original' / 'owner.json').exists())
        self.assertTrue((self.state / 'partial').exists())

    def test_runtime_cleanup_failure_keeps_recovery_material(self):
        self.capture()
        self.assertTrue(callable(getattr(backup, 'finish_cleanup', None)),
                        'failure-aware checkpoint lifecycle missing')
        def fail(): raise RuntimeError('runtime cleanup sentinel')
        with self.assertRaisesRegex(RuntimeError, 'runtime cleanup sentinel'):
            backup.finish_cleanup(True, fail, lambda: shutil.rmtree(self.archive))
        self.assertTrue(self.archive.exists())

    def test_successful_recovery_cleans_only_owned_fixture_material(self):
        self.capture()
        self.assertTrue(callable(getattr(backup, 'finish_cleanup', None)))
        backup.finish_cleanup(True, lambda: None, lambda: shutil.rmtree(self.archive))
        self.assertFalse(self.archive.exists()); self.assertTrue(self.state.exists())


if __name__ == '__main__':
    unittest.main()
