# SPDX-License-Identifier: GPL-3.0-or-later
"""Private native-probe helper, NOT a general/privileged backup product API.

Caller exclusively owns this disposable fixture and quarantines originals while
stopped. No rollback or migration is implemented; original paths are mandatory.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat

STATE = {'owner.json', 'desired.json', 'owner.json.lock', 'desired.json.lock'}


def require_stopped(stopped):
    if stopped() is not True:
        raise ValueError('exact Agent unit must be inactive with no main process')


def private_directory(path):
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise ValueError('private owned directory required')


def record(path, secret):
    info = path.lstat()
    if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid != os.geteuid()
            or info.st_mode & (0o077 if secret else 0o022)):
        raise ValueError('owned regular independent file and safe permissions required')
    return {'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'mode': stat.S_IMODE(info.st_mode), 'uid': info.st_uid, 'gid': info.st_gid}


def paths(state, token, unit):
    values = [Path(value) for value in [state, token, unit]]
    if any(not value.is_absolute() or '..' in value.parts for value in values):
        raise ValueError('exact absolute original paths required')
    return values


def files(state, token, unit):
    private_directory(state)
    if {path.name for path in state.iterdir()} != STATE:
        raise ValueError('complete known retained state required')
    return {**{'state/' + name: state / name for name in sorted(STATE)},
            'token': token, 'unit': unit}


def capture(state, token, unit, backup, stopped):
    require_stopped(stopped)
    state, token, unit = paths(state, token, unit)
    private_directory(backup.parent)
    sources = files(state, token, unit)
    manifest = {'paths': list(map(str, [state, token, unit])),
                'files': {name: record(path, name != 'unit') for name, path in sources.items()}}
    require_stopped(stopped)
    backup.mkdir(mode=0o700)  # Never overwrite an existing checkpoint.
    (backup / 'state').mkdir(mode=0o700)
    for name, source in sources.items():
        shutil.copy2(source, backup / name)
        if record(backup / name, name != 'unit') != manifest['files'][name]:
            raise ValueError('checkpoint copy differs')
    require_stopped(stopped)
    # Detect an unexpected mutation while copying, not just a self-consistent copy.
    if any(record(path, name != 'unit') != manifest['files'][name] for name, path in sources.items()):
        raise ValueError('stopped checkpoint source changed')
    fd = os.open(backup / 'manifest.json', os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(fd, 'w') as output:
        json.dump(manifest, output)
    return manifest


def restore(state, token, unit, backup, stopped):
    require_stopped(stopped)
    state, token, unit = paths(state, token, unit)
    private_directory(backup)
    if {path.name for path in backup.iterdir()} != {'state', 'token', 'unit', 'manifest.json'}:
        raise ValueError('complete private checkpoint required')
    record(backup / 'manifest.json', True)
    manifest = json.loads((backup / 'manifest.json').read_text())
    if manifest.get('paths') != list(map(str, [state, token, unit])):
        raise ValueError('checkpoint belongs to different original paths')
    sources = files(backup / 'state', backup / 'token', backup / 'unit')
    actual = {name: record(path, name != 'unit') for name, path in sources.items()}
    if manifest.get('files') != actual:
        raise ValueError('checkpoint contents or permissions changed')
    if any(os.path.lexists(value) for value in [state, token, unit]):
        raise ValueError('restore never overwrites existing destinations')
    require_stopped(stopped)
    state.mkdir(mode=0o700)
    targets = {**{'state/' + name: state / name for name in sorted(STATE)},
               'token': token, 'unit': unit}
    for name, source in sources.items():
        # Exclusive target creation; no accidental overwrite even on a late race.
        with source.open('rb') as source_file, targets[name].open('xb') as target_file:
            shutil.copyfileobj(source_file, target_file)
        shutil.copystat(source, targets[name])
        if record(targets[name], name != 'unit') != actual[name]:
            raise ValueError('restored contents or permissions differ')
        if targets[name].stat().st_ino == source.stat().st_ino:
            raise ValueError('restore must copy, not reuse source inode')
    require_stopped(stopped)
    return {'files': 6, 'byteIdentical': True, 'independentCopies': True,
            'private': True, 'originalPaths': True, 'stopped': True}
