#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Opt-in real Controller-mode Agent service; public evidence never includes state."""
import argparse
import copy
import ctypes
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import struct
import subprocess
import sys
import tempfile
import time
import threading
import unittest
import urllib.request
from stopped_agent_state import capture, restore, quarantine_state, finish_cleanup

ROOT = Path(__file__).resolve().parents[1]
CORE = '64c8cb8e6c92669873031ca45cff47dbb38d0224'
TREE = '4637c029a34c71ec04b66fcc205c288f1280dfb9'
PHASES = {'startup': True, 'crash': True, 'offline': True, 'stopped': True, 'restored': True,
          'expired': False, 'restoredExpired': False, 'offlineRestart': False, 'reconnected': False}
PHASE_PROCESS = {'startup': 0, 'crash': 1, 'offline': 1, 'stopped': None, 'restored': 2,
                 'expired': 2, 'restoredExpired': None, 'offlineRestart': 3, 'reconnected': 3}


def controller_receipt(nodes, authority):
    node = next((n for n in nodes if n['agent_ref'] == 'node'), None)
    if not node or not node.get('report') or node['report'].get('phase') != 'APPLIED' or not node.get('matches_desired'):
        return False
    if node.get('connection') != 'connected' or node.get('freshness') != 'fresh':
        return False
    assert int(node['desired_revision']) == int(node['report']['revision']) == 2
    assert node['desired_sha256'] == node['report']['sha256']
    return {'authority': authority, 'revision': 2, 'desiredSHA256': node['desired_sha256'],
            'reportSHA256': node['report']['sha256'], 'phase': 'APPLIED',
            'connection': node['connection'], 'freshness': node['freshness'], 'matchesDesired': True}


def validate_report(report):
    def keys(value, expected):
        assert isinstance(value, dict) and set(value) == set(expected), 'public evidence fields'
    def sha(value):
        assert isinstance(value, str) and re.fullmatch('[a-f0-9]{64}', value)
    keys(report, ['result', 'coreCommit', 'coreTree', 'agentSHA256', 'controllerSHA256',
                  'authority', 'processes', 'namespace', 'cache', 'timeline', 'receipts',
                  'observations', 'offlineState', 'reconnectedState', 'backup'])
    assert report['result'] == 'pass'
    assert report['coreCommit'] == CORE and report['coreTree'] == TREE
    sha(report['agentSHA256']); sha(report['controllerSHA256'])
    assert re.fullmatch('controller:[a-f0-9]{32}', report['authority'])
    assert isinstance(report['namespace'], int) and report['namespace'] > 0
    assert len(report['processes']) == 4
    pids = set()
    for process in report['processes']:
        keys(process, ['pid', 'capabilities', 'noNewPrivileges', 'binarySHA256', 'namespace'])
        assert isinstance(process['pid'], int) and process['pid'] > 0 and process['pid'] not in pids
        pids.add(process['pid'])
        assert process['capabilities'] == 4096 and process['noNewPrivileges'] == 1
        assert process['binarySHA256'] == report['agentSHA256']
        assert process['namespace'] == report['namespace']
    keys(report['cache'], ['before', 'after'])
    sha(report['cache']['before']); assert report['cache']['after'] == report['cache']['before']
    timeline = report['timeline']; keys(timeline, ['published', 'deadline', 'offline', 'expired'])
    assert all(isinstance(value, (int, float)) for value in timeline.values())
    assert 30 <= timeline['deadline'] - timeline['published'] <= 90
    assert timeline['published'] < timeline['offline'] < timeline['deadline'] <= timeline['expired']
    backup = report['backup']
    keys(backup, ['files', 'byteIdentical', 'independentCopies', 'private',
                  'originalPaths', 'stopped', 'captured', 'restored'])
    assert type(backup['files']) is int and backup['files'] == 6
    for key in ['byteIdentical', 'independentCopies', 'private', 'originalPaths', 'stopped']:
        assert backup[key] is True
    assert timeline['offline'] <= backup['captured'] <= backup['restored'] < timeline['deadline']
    assert len(report['receipts']) == 2
    for receipt in report['receipts']:
        keys(receipt, ['authority', 'revision', 'desiredSHA256', 'reportSHA256', 'phase',
                       'connection', 'freshness', 'matchesDesired'])
        assert receipt['authority'] == report['authority'] and receipt['revision'] == 2
        sha(receipt['desiredSHA256']); assert receipt['reportSHA256'] == receipt['desiredSHA256']
        assert receipt['phase'] == 'APPLIED' and receipt['connection'] == 'connected'
        assert receipt['freshness'] == 'fresh' and receipt['matchesDesired'] is True
    assert report['receipts'][0]['desiredSHA256'] == report['receipts'][1]['desiredSHA256']
    expected = {(phase, family, protocol) for phase in PHASES for family in [4, 6] for protocol in ['tcp', 'udp']}
    assert len(report['observations']) == len(expected)
    for observation in report['observations']:
        keys(observation, ['phase', 'family', 'protocol', 'allow', 'health', 'agentRunning', 'pid'])
        key = (observation['phase'], observation['family'], observation['protocol'])
        assert key in expected; expected.remove(key)
        assert observation['allow'] is PHASES[observation['phase']] and observation['health'] is True
        process_index = PHASE_PROCESS[observation['phase']]
        assert observation['agentRunning'] is (process_index is not None)
        assert type(observation['pid']) is int
        assert observation['pid'] == (0 if process_index is None else report['processes'][process_index]['pid'])
    assert not expected
    assert report['offlineState'] == 'activating' and report['reconnectedState'] == 'active'


def command(*args, **options):
    return subprocess.run(args, check=True, text=True, capture_output=True, timeout=30, **options)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def stage_agent(binary, work):
    # ProtectHome=yes hides GitHub's /home/runner checkout from the service.
    # Preserve the actual verified artifact, not a wrapper or relaxed sandbox.
    staged = work / 'picketxd'
    shutil.copy2(binary, staged)
    assert digest(staged) == digest(binary)
    return staged


def stop_process(child, timeout=10):
    if child is not None and child.poll() is None:
        child.terminate()
        try:
            child.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            child.kill(); child.wait(timeout=5)


def start_process_group(argv, **options):
    # Linux test driver only: adopt/reap our orphaned descendants after a worker
    # timeout, instead of relying on the runner's PID1 to reap them later.
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(36, 1, 0, 0, 0) != 0:  # PR_SET_CHILD_SUBREAPER
        raise OSError(ctypes.get_errno(), 'set probe child subreaper')
    return subprocess.Popen(argv, start_new_session=True, **options)


def stop_process_group(child, timeout=1):
    # Only a Popen started with start_new_session=True is passed here. Its group
    # includes Controller/peer descendants, never the manager-owned Agent.
    def send(sig):
        try:
            os.killpg(child.pid, sig)
        except ProcessLookupError:
            pass
    send(signal.SIGTERM)
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        try:
            os.killpg(child.pid, 0)
        except ProcessLookupError:
            break
        time.sleep(0.02)
    send(signal.SIGKILL)
    child.wait(timeout=5)
    end = time.monotonic() + 5
    while True:
        try:
            pid, _ = os.waitpid(-child.pid, os.WNOHANG)
        except ChildProcessError:
            break
        if pid:
            continue
        if time.monotonic() >= end:
            raise TimeoutError('owned worker descendants not reaped')
        time.sleep(0.02)


def cleanup_steps(*actions):
    errors = []
    for action in actions:
        try:
            action()
        except Exception as error:
            errors.append(error)
    if errors:
        # If invoked by finally, Python retains the original failure as context.
        raise ExceptionGroup('owned probe cleanup failed', errors)


def begin_report(artifacts):
    artifacts.mkdir(parents=True, exist_ok=True)
    (artifacts / 'controller-service-report.json').unlink(missing_ok=True)


def start_ready_process(argv, ready, **options):
    child = subprocess.Popen(argv, **options)
    try:
        wait_for(lambda: ready(child), 'real Controller ready')
        return child
    except BaseException:
        stop_process(child)
        raise


def wait_for(check, label, limit=25):
    end = time.monotonic() + limit
    while time.monotonic() < end:
        value = check()
        if value:
            return value
        time.sleep(0.2)
    raise AssertionError('timed out: ' + label)


class EchoSupervisor:
    def __init__(self):
        self.failed = threading.Event()
        self.threads = []

    def start(self, target, *args):
        def guarded():
            try:
                target(*args)
            except Exception:
                pass  # Private handler details never enter public evidence.
            finally:
                self.failed.set()
        thread = threading.Thread(target=guarded, daemon=True)
        self.threads.append(thread)
        thread.start()

    def healthy(self):
        return not self.failed.is_set() and len(self.threads) == 6 and all(t.is_alive() for t in self.threads)


def echo_connection(conn, healthy=None):
    try:
        conn.settimeout(1)
        data = conn.recv(32)
        if data:
            conn.sendall(data if healthy is None or healthy() else b'peer-unhealthy')
    except (ConnectionError, TimeoutError):
        # Ordinary reset/disconnect cannot terminate the protected listener.
        pass


def serve():
    supervisor = EchoSupervisor()
    def echo(sock, tcp, health):
        while True:
            if tcp:
                conn, _ = sock.accept()
                with conn:
                    echo_connection(conn, supervisor.healthy if health else None)
            else:
                try:
                    data, peer = sock.recvfrom(32); sock.sendto(data, peer)
                except ConnectionError:
                    continue
    for family, host in [(socket.AF_INET, '127.0.0.1'), (socket.AF_INET6, '::1')]:
        for tcp, port in [(True, 18443), (False, 18444), (True, 18449)]:
            sock = socket.socket(family, socket.SOCK_STREAM if tcp else socket.SOCK_DGRAM)
            if family == socket.AF_INET6:
                sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 1)
            sock.bind((host, port))
            if tcp:
                sock.listen(16)
            supervisor.start(echo, sock, tcp, port == 18449)
    threading.Event().wait()


def worker(args, work, token):
    assert os.stat('/proc/self/ns/net').st_ino != os.stat('/proc/1/ns/net').st_ino
    binary, bundle = stage_agent(args.binary.resolve(), work), args.controller_bundle.resolve()
    source = json.loads((bundle / 'SOURCE.json').read_text())
    assert source == {'commit': CORE, 'tree': TREE, 'platform': 'linux-amd64'}
    state = Path('/var/lib') / token
    instance = work / 'controller'
    controller_binary = bundle / 'bin/picketx-controller'
    command(str(bundle / 'bin/picketx'), 'controller-init', '--directory', str(instance),
            '--assets', str(bundle / 'web'), '--origin', 'http://127.0.0.1:18080',
            '--listen', '127.0.0.1:18080', '--agent-listen', '127.0.0.1:19090',
            '--authz-listen', '127.0.0.1:19091', '--agent-ref', 'node', env={'PATH': ''})
    config_path = instance / 'controller.yaml'
    config = json.loads(config_path.read_text()); authority = config['agent']['authority']
    admin_token = (instance / 'bootstrap-key').read_text().strip()
    unit = token + '.service'; unit_path = Path('/run/systemd/system') / unit
    report = {'result': 'fail', 'coreCommit': CORE, 'coreTree': TREE,
              'agentSHA256': digest(binary), 'controllerSHA256': digest(controller_binary),
              'authority': authority, 'namespace': os.stat('/proc/self/ns/net').st_ino,
              'processes': [], 'observations': [], 'receipts': [], 'cache': {}, 'timeline': {}}
    controller = peer = None
    unit_owned = False

    def ctl(*values):
        return command('systemctl', *values)
    def prop(name):
        return ctl('show', unit, '--property=' + name, '--value').stdout.strip()
    def api(path, body=None, revision=None):
        headers = {'Authorization': 'Bearer ' + admin_token}
        if revision is not None:
            headers['If-Match'] = '"' + str(revision) + '"'
        data = None if body is None else json.dumps(body).encode()
        if data is not None:
            headers['Content-Type'] = 'application/json'
        request = urllib.request.Request('http://127.0.0.1:18080/api/v1/' + path,
                                         data=data, headers=headers, method='GET' if body is None else 'PUT')
        with urllib.request.urlopen(request, timeout=2) as response:
            assert response.status == 200
            return json.load(response)
    def start_controller():
        def ready(child):
            assert child.poll() is None, 'actual Controller must remain alive'
            try:
                with urllib.request.urlopen('http://127.0.0.1:18080/readyz', timeout=0.5) as response:
                    return response.status == 200
            except OSError:
                return False
        with (work / 'controller.log').open('ab') as log:
            return start_ready_process([str(controller_binary), '-config', str(config_path)],
                                       ready, cwd=work, env={'PATH': ''}, stdout=log,
                                       stderr=subprocess.STDOUT)
    def receipt():
        return controller_receipt(api('agent-observations')['nodes'], authority)
    def status(predicate, old_pid=0):
        def check():
            pid = int(prop('MainPID'))
            if not pid or pid == old_pid:
                return False
            journal = command('journalctl', '--unit=' + unit, '--no-pager', '--output=json').stdout
            values = []
            for line in journal.splitlines():
                entry = json.loads(line)
                if entry.get('_PID') != str(pid):
                    continue
                try:
                    value = json.loads(entry.get('MESSAGE', ''))
                except (ValueError, TypeError):
                    continue
                if isinstance(value, dict) and 'enforcementConfirmed' in value:
                    values.append(value)
            return (pid, values[-1]) if values and predicate(values[-1]) else False
        return wait_for(check, 'fresh actual Agent process status')
    def process(pid):
        fields = dict(line.split(':', 1) for line in Path(f'/proc/{pid}/status').read_text().splitlines() if ':' in line)
        actual = {'pid': pid, 'capabilities': int(fields['CapEff'], 16),
                  'noNewPrivileges': int(fields['NoNewPrivs']),
                  'binarySHA256': digest(f'/proc/{pid}/exe'),
                  'namespace': os.stat(f'/proc/{pid}/ns/net').st_ino}
        assert actual['binarySHA256'] == report['agentSHA256']
        assert actual['namespace'] == report['namespace']
        assert actual['capabilities'] == 4096 and actual['noNewPrivileges'] == 1
        report['processes'].append(actual)
    def echo(family, protocol, port):
        with socket.socket(socket.AF_INET if family == 4 else socket.AF_INET6,
                           socket.SOCK_STREAM if protocol == 'tcp' else socket.SOCK_DGRAM) as sock:
            sock.settimeout(0.6)
            try:
                sock.connect(('127.0.0.1' if family == 4 else '::1', port))
                sock.sendall(b'service-proof')
                return sock.recv(32) == b'service-proof'
            except OSError:
                return False
    def health():
        assert peer.poll() is None, 'packet peer must stay alive'
        assert all(echo(family, 'tcp', 18449) for family in [4, 6]), 'independent unprotected dual-stack health'
    def packets(phase):
        health()
        phase_pid = int(prop('MainPID'))
        assert bool(phase_pid) == (PHASE_PROCESS[phase] is not None), 'actual phase process state'
        for family in [4, 6]:
            for protocol, port in [('tcp', 18443), ('udp', 18444)]:
                allow = echo(family, protocol, port)
                assert allow == PHASES[phase], (phase, family, protocol, 'actual packet outcome mismatch')
                report['observations'].append({'phase': phase, 'family': family,
                                               'protocol': protocol, 'allow': allow, 'health': True,
                                               'agentRunning': bool(phase_pid), 'pid': phase_pid})
        health()
    try:
        peer = subprocess.Popen([sys.executable, '-B', str(Path(__file__).resolve()), '--serve'],
                                stdout=(work / 'peer.log').open('ab'), stderr=subprocess.STDOUT)
        wait_for(lambda: all(echo(family, 'tcp', 18449) for family in [4, 6]), 'actual packet peer')
        controller = start_controller()
        policy = (ROOT / 'examples/enforce/policy.yaml').read_text()
        # Valid static deny sentinel: accidentally using file mode cannot pass.
        sentinel = '---\n'.join(document for document in policy.split('---\n') if 'kind: Grant\n' not in document)
        (work / 'policy.yaml').write_text(sentinel)
        now = time.time(); deadline = now + 60
        expiry = dt.datetime.fromtimestamp(deadline, dt.timezone.utc).isoformat().replace('+00:00', 'Z')
        policy = policy.replace('validity: {mode: permanent}', "validity: {mode: bounded, expiresAt: '" + expiry + "'}")
        facts = {'authorityID': authority, 'agents': [{'Name': 'node', 'Labels': {},
                 'Capabilities': {'Classes': ['host-local'], 'Protocols': ['tcp', 'udp'],
                                  'Families': [4, 6], 'Handling': ['graceful'], 'OriginalDestination': True}}],
                 'contexts': [{'AgentRef': 'node', 'ProfileRef': 'host', 'ID': 'remote-service',
                               'NetworkSpaceID': 'not-a-local-identity', 'Class': 'host-local',
                               'Stage': 'original-destination', 'Handling': 'graceful',
                               'Destinations': ['127.0.0.1/32', '::1/128']}]}
        current = api('publication')['revision']
        assert current == 1, 'private init publishes explicit empty revision1'
        published = api('publication', {'policy': policy, 'facts': json.dumps(facts),
                                       'reason': 'Actual Agent service bounded policy'}, current)
        assert published['revision'] == 2
        report['timeline'].update(published=now, deadline=deadline)
        service = args.unit.read_text()
        for old, new in [('/usr/local/libexec/picketxd', str(binary)),
                         ('/etc/picketx/policy.yaml', str(work / 'policy.yaml')),
                         ('/etc/picketx/agent-token', str(instance / 'agent-token')),
                         ('/var/lib/picketx', str(state)), ('StateDirectory=picketx', 'StateDirectory=' + token),
                         ('--provider static-demo', '--provider ' + authority),
                         ('--provider controller:REPLACE_WITH_GENERATED_AUTHORITY', '--provider ' + authority),
                         ('127.0.0.1:9090', '127.0.0.1:19090'), ('--poll 5s', '--poll 200ms')]:
            service = service.replace(old, new)
        service = service.replace('[Service]\n', '[Service]\nNetworkNamespacePath=/run/netns/' + token + '\n')
        with unit_path.open('x') as file:
            unit_owned = True; file.write(service)
        (work / 'unit-owned').touch()
        command('systemd-analyze', 'verify', '--man=no', '--generators=no', str(unit_path))
        ctl('daemon-reload'); ctl('start', unit)
        report['receipts'].append(wait_for(receipt, 'actual connected Controller APPLIED receipt'))
        pid, first = status(lambda value: value['enforcementConfirmed'] and not value['desired'].get('lastError') and value['desired'].get('remoteState') == 'current')
        assert prop('ActiveState') == 'active' and prop('Type') == 'notify'
        process(pid); packets('startup')
        report['cache']['before'] = digest(state / 'desired.json')
        ctl('kill', '--signal=SIGKILL', '--kill-whom=main', unit)
        pid, recovered = status(lambda value: value['enforcementConfirmed'] and value['desired']['restored'] and not value['desired'].get('lastError') and value['desired'].get('remoteState') == 'current', pid)
        ctl('start', unit)
        assert prop('ActiveState') == 'active'
        assert int(prop('NRestarts')) >= 1
        assert recovered['desired']['generation'] == first['desired']['generation'] == 2
        assert recovered['appliedPolicyHash'] == first['appliedPolicyHash']
        process(pid); packets('crash')
        stop_process(controller)
        assert controller.poll() is not None, 'actual Controller process stopped'
        status(lambda value: value['desired'].get('remoteState') == 'disconnected')
        report['timeline']['offline'] = time.time(); assert time.time() < deadline
        packets('offline')
        # Latest checkpoint only, no newer publication/revocation has occurred.
        ctl('stop', unit)
        def stopped():
            return prop('ActiveState') == 'inactive' and prop('MainPID') == '0'
        assert stopped(), 'actual exact Agent stopped before filesystem capture'
        packets('stopped')
        checkpoint = work / 'agent-checkpoint'
        credential = instance / 'agent-token'
        captured = time.time(); assert captured < deadline
        capture(state, credential, unit_path, checkpoint, stopped)
        # Recoverable quarantine: never erase the prior files to manufacture success.
        quarantine_state(state)
        credential.rename(work / 'token-original')
        unit_path.rename(work / 'unit-original')
        proof = restore(state, credential, unit_path, checkpoint, stopped)
        proof.update(captured=captured, restored=time.time())
        assert proof['restored'] < deadline
        report['backup'] = proof
        ctl('daemon-reload')
        assert stopped(), 'restore must not implicitly start the unit'
        ctl('start', '--no-block', unit)
        pid, restored = status(lambda value: value['enforcementConfirmed'] and value['desired']['restored'] and value['desired'].get('remoteState') in ['awaiting', 'disconnected'], pid)
        assert restored['desired']['generation'] == 2 and prop('ActiveState') == 'activating'
        process(pid); packets('restored')
        time.sleep(max(0, deadline + 0.2 - time.time()))
        # Keep the former continuous running/disconnected-across-expiry gate.
        assert int(prop('MainPID')) == pid
        status(lambda value: value['desired'].get('remoteState') in ['awaiting', 'disconnected'])
        report['timeline']['expired'] = time.time(); packets('expired')
        ctl('stop', unit)
        assert stopped(); packets('restoredExpired')
        ctl('start', '--no-block', unit)
        pid, expired = status(lambda value: value['enforcementConfirmed'] and value['desired']['restored'] and value['desired'].get('remoteState') in ['awaiting', 'disconnected'], pid)
        assert expired['desired']['generation'] == 2
        process(pid); packets('offlineRestart')
        report['offlineState'] = prop('ActiveState'); assert report['offlineState'] == 'activating'
        controller = start_controller()
        ctl('start', unit)
        status(lambda value: value['enforcementConfirmed'] and not value['desired'].get('lastError') and value['desired'].get('remoteState') == 'current')
        report['receipts'].append(wait_for(receipt, 'fresh reconnected Controller APPLIED receipt'))
        report['reconnectedState'] = prop('ActiveState')
        packets('reconnected'); report['cache']['after'] = digest(state / 'desired.json')
        assert api('publication')['revision'] == 2, 'recovery must not publish/renew authority'
        report['result'] = 'pass'; validate_report(report)
        (work / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
    finally:
        cleanup_steps(lambda: command('systemctl', 'stop', unit) if unit_owned else None,
                      lambda: stop_process(controller), lambda: stop_process(peer))


def acceptance(args):
    assert os.environ.get('PICKETX_CONTROLLER_SERVICE_TESTS') == '1', 'explicit native opt-in required'
    assert os.geteuid() == 0, 'actual system manager and fresh namespace require root'
    if args.worker:
        work = args.worker.resolve(); assert work.parent == Path('/run') and work.name.startswith('picketx-remote-service-')
        worker(args, work, work.name); return
    begin_report(args.artifacts)
    command('systemctl', 'show', '--property=Version')
    work = Path(tempfile.mkdtemp(prefix='picketx-remote-service-', dir='/run')); token = work.name
    created = False
    child = None
    completed_report = None
    try:
        command('ip', 'netns', 'add', token); created = True
        command('ip', '-n', token, 'link', 'set', 'lo', 'up')
        # Keep public stdout body-free; private process logs/state stay in work.
        child = start_process_group(['ip', 'netns', 'exec', token, sys.executable, '-B', str(Path(__file__).resolve()),
                        '--worker', str(work), '--binary', str(args.binary.resolve()),
                        '--controller-bundle', str(args.controller_bundle.resolve()),
                        '--artifacts', str(args.artifacts.resolve()), '--unit', str(args.unit.resolve())],
                                 )
        result = child.wait(timeout=150)
        if result:
            raise subprocess.CalledProcessError(result, child.args)
        candidate_report = json.loads((work / 'report.json').read_text())
        validate_report(candidate_report)
        completed_report = candidate_report
    finally:
        owned = (work / 'unit-owned').exists()
        def stop_owned_unit():
            if owned:
                try:
                    command('systemctl', 'stop', token + '.service')
                except Exception:
                    # Exact fixture unit only; never kill a host/global service.
                    command('systemctl', 'kill', '--kill-whom=all', '--signal=SIGKILL', token + '.service')
                    command('systemctl', 'stop', token + '.service')
                    raise
        def runtime_cleanup():
            cleanup_steps(stop_owned_unit,
            lambda: stop_process_group(child) if child is not None else None,
            # A successful stop can unload this ephemeral unit. reset-failed is
            # unnecessary and rejects that ordinary absence on real systemd.
            lambda: (Path('/run/systemd/system') / (token + '.service')).unlink(missing_ok=True) if owned else None,
            lambda: command('systemctl', 'daemon-reload') if owned else None,
            lambda: command('ip', 'netns', 'delete', token) if created else None)
        def private_cleanup():
            state = Path('/var/lib') / token
            saved = state.with_name(state.name + '.stopped-original')
            cleanup_steps(lambda: shutil.rmtree(state) if state.exists() else None,
                          lambda: shutil.rmtree(saved) if saved.exists() else None,
                          lambda: shutil.rmtree(work))
        try:
            finish_cleanup(completed_report is not None, runtime_cleanup, private_cleanup)
        finally:
            if work.exists():
                # Paths only, never private bodies. Disposable-runner root may
                # inspect then remove these exact owned paths after confirming
                # no fixture unit/process/namespace remains; never restore them
                # into a replacement namespace or upload them as evidence.
                saved = (Path('/var/lib') / token).with_name(token + '.stopped-original')
                print(f'Private failed-fixture material retained for root inspection only: {work}; /var/lib/{token}; {saved}', file=sys.stderr)
    (args.artifacts / 'controller-service-report.json').write_text(json.dumps(completed_report, indent=2) + '\n')
    print('PASS actual Controller-mode Agent systemd: remote APPLIED, crash LKG, stopped six-file original-path backup/restore, offline absolute expiry, offline readiness and reconnect')


class EvidenceTests(unittest.TestCase):
    def test_expired_phase_keeps_independent_running_agent_proof(self):
        observation = next(o for o in self.fixture()['observations'] if o['phase'] == 'expired')
        self.assertIs(observation.get('agentRunning'), True,
                      'prior running-disconnected-across-deadline proof missing')

    def test_old_restart_only_evidence_is_not_complete_backup_proof(self):
        report = self.fixture()
        report.pop('backup', None)
        with self.assertRaises(AssertionError):
            validate_report(report)

    def test_failed_protected_worker_invalidates_independent_health(self):
        stop = threading.Event()
        fail = threading.Event()
        supervisor = EchoSupervisor()
        def protected():
            fail.wait()
            raise RuntimeError('protected handler sentinel')
        supervisor.start(protected)
        for _ in range(5):
            supervisor.start(stop.wait)
        try:
            self.assertTrue(supervisor.healthy())
            fail.set()
            supervisor.threads[0].join(timeout=1)
            self.assertTrue(all(t.is_alive() for t in supervisor.threads[1:]))
            self.assertFalse(supervisor.healthy())
        finally:
            fail.set(); stop.set()
            for thread in supervisor.threads:
                thread.join(timeout=1)

    def test_actual_tcp_reset_does_not_kill_protected_handler(self):
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0)); listener.listen(2); listener.settimeout(2)
            errors = []
            def protected():
                try:
                    for _ in range(2):
                        conn, _ = listener.accept()
                        with conn:
                            echo_connection(conn)
                except BaseException as error:
                    errors.append(error)
            thread = threading.Thread(target=protected, daemon=True); thread.start()
            with socket.create_connection(listener.getsockname(), timeout=1) as client:
                client.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack('ii', 1, 0))
            with socket.create_connection(listener.getsockname(), timeout=1) as client:
                client.sendall(b'second'); self.assertEqual(client.recv(32), b'second')
            thread.join(timeout=2)
            self.assertFalse(thread.is_alive()); self.assertEqual(errors, [])

    def test_cleanup_continues_after_unit_stop_timeout(self):
        child = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(60)'])
        def timeout():
            raise subprocess.TimeoutExpired('owned unit stop', 0.01)
        try:
            with self.assertRaises(ExceptionGroup):
                cleanup_steps(timeout, lambda: stop_process(child))
            self.assertIsNotNone(child.poll())
        finally:
            if child.poll() is None:
                child.kill(); child.wait(timeout=5)

    def test_outer_timeout_terminates_actual_owned_worker_descendants(self):
        child = start_process_group([sys.executable, '-u', '-c',
            'import subprocess,sys,time; p=subprocess.Popen([sys.executable,"-u","-c","import signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); print(1); time.sleep(60)"],stdout=subprocess.PIPE,text=True); p.stdout.readline(); print(p.pid); time.sleep(60)'],
            stdout=subprocess.PIPE, text=True)
        descendant = int(child.stdout.readline())
        try:
            with self.assertRaises(subprocess.TimeoutExpired):
                child.wait(timeout=0.05)
            stop_process_group(child, timeout=0.1)
            self.assertIsNotNone(child.poll())
            self.assertFalse(Path(f'/proc/{descendant}').exists(), 'owned descendant fully reaped')
        finally:
            os.killpg(child.pid, signal.SIGKILL) if child.poll() is None else None
            child.wait(timeout=5); child.stdout.close()

    def test_new_run_removes_only_previous_report(self):
        with tempfile.TemporaryDirectory() as temporary:
            artifacts = Path(temporary)
            report = artifacts / 'controller-service-report.json'
            report.write_text('{"result":"pass"}')
            sentinel = artifacts / 'operator-data'; sentinel.write_text('keep')
            begin_report(artifacts)
            self.assertFalse(report.exists()); self.assertEqual(sentinel.read_text(), 'keep')

    def test_failed_readiness_reaps_actual_child(self):
        children = []
        def reject(child):
            children.append(child)
            raise ValueError('readiness sentinel')
        with self.assertRaisesRegex(ValueError, 'readiness sentinel'):
            start_ready_process([sys.executable, '-c', 'import time; time.sleep(60)'], reject)
        self.assertIsNotNone(children[0].poll())

    def test_cleanup_reaps_actual_sigterm_resistant_child(self):
        child = subprocess.Popen([sys.executable, '-u', '-c',
            'import signal,time; signal.signal(signal.SIGTERM, signal.SIG_IGN); print("ready"); time.sleep(60)'],
            stdout=subprocess.PIPE, text=True)
        try:
            self.assertEqual(child.stdout.readline().strip(), 'ready')
            stop_process(child, timeout=0.05)
            self.assertIsNotNone(child.poll())
        finally:
            if child.poll() is None:
                child.kill(); child.wait(timeout=5)
            child.stdout.close()

    def test_stage_actual_binary_outside_protected_home(self):
        with tempfile.TemporaryDirectory() as temporary:
            work = Path(temporary)
            original = Path('/usr/bin/true').resolve()
            staged = stage_agent(original, work)
            self.assertEqual(staged, work / 'picketxd')
            self.assertNotEqual(staged.stat().st_ino, original.stat().st_ino)
            self.assertEqual(digest(staged), digest(original))
            subprocess.run([str(staged)], env={'PATH': ''}, check=True)

    def test_real_pending_controller_report_is_not_applied(self):
        self.assertFalse(controller_receipt([{'agent_ref': 'node', 'report': None}], 'controller:pending'))

    def fixture(self):
        return {'result': 'pass', 'coreCommit': CORE, 'coreTree': TREE,
                'agentSHA256': 'a' * 64, 'controllerSHA256': 'b' * 64,
                'authority': 'controller:' + 'c' * 32,
                'processes': [{'pid': pid, 'capabilities': 4096, 'noNewPrivileges': 1,
                               'binarySHA256': 'a' * 64, 'namespace': 99} for pid in [11, 22, 33, 44]],
                'namespace': 99, 'cache': {'before': 'd' * 64, 'after': 'd' * 64},
                'timeline': {'published': 100, 'deadline': 160, 'offline': 110, 'expired': 161},
                'backup': {'files': 6, 'byteIdentical': True, 'independentCopies': True,
                           'private': True, 'originalPaths': True, 'stopped': True,
                           'captured': 115, 'restored': 120},
                'receipts': [{'authority': 'controller:' + 'c' * 32, 'revision': 2,
                              'desiredSHA256': 'e' * 64, 'reportSHA256': 'e' * 64,
                              'phase': 'APPLIED', 'connection': 'connected',
                              'freshness': 'fresh', 'matchesDesired': True} for _ in range(2)],
                'observations': [{'phase': phase, 'family': family, 'protocol': protocol,
                                  'allow': allow, 'health': True,
                                  'agentRunning': phase not in ['stopped', 'restoredExpired'],
                                  'pid': {'startup':11, 'crash':22, 'offline':22, 'stopped':0,
                                          'restored':33, 'expired':33, 'restoredExpired':0,
                                          'offlineRestart':44, 'reconnected':44}[phase]}
                                 for phase, allow in PHASES.items()
                                 for family in [4, 6] for protocol in ['tcp', 'udp']],
                'offlineState': 'activating', 'reconnectedState': 'active'}

    def test_valid(self):
        validate_report(self.fixture())

    def test_mutations_rejected(self):
        mutations = [lambda r: r['processes'][1].update(pid=11),
                     lambda r: r['backup'].update(files=5),
                     lambda r: r['backup'].update(stopped=False),
                     lambda r: r['backup'].update(byteIdentical=False),
                     lambda r: r['backup'].update(independentCopies=False),
                     lambda r: r['backup'].update(private=False),
                     lambda r: r['backup'].update(originalPaths=False),
                     lambda r: r['backup'].update(restored=170),
                     lambda r: r['backup'].update(token='secret'),
                     lambda r: r['processes'][0].update(capabilities=0),
                     lambda r: r['processes'][0].update(binarySHA256='f' * 64),
                     lambda r: r['cache'].update(after='f' * 64),
                     lambda r: r['timeline'].update(expired=159),
                     lambda r: r.update(offlineState='active'),
                     lambda r: r['receipts'][1].update(matchesDesired=False),
                     lambda r: r['receipts'][1].update(revision=1),
                     lambda r: r['receipts'][1].update(reportSHA256='f' * 64),
                     lambda r: r['observations'][-1].update(allow=True),
                     lambda r: r['observations'][0].update(health=False),
                     lambda r: r['observations'][20].update(agentRunning=False),
                     lambda r: r['observations'][20].update(pid=0),
                     lambda r: r['observations'].pop(),
                     lambda r: r.update(token='secret-sentinel')]
        for mutate in mutations:
            with self.subTest(mutation=mutations.index(mutate)):
                report = copy.deepcopy(self.fixture()); mutate(report)
                with self.assertRaises(AssertionError):
                    validate_report(report)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--self-test', action='store_true')
    parser.add_argument('--serve', action='store_true')
    parser.add_argument('--worker', type=Path)
    parser.add_argument('--binary', type=Path)
    parser.add_argument('--controller-bundle', type=Path)
    parser.add_argument('--artifacts', type=Path)
    parser.add_argument('--unit', type=Path, default=ROOT / 'deploy/systemd/picketxd-controller.service')
    args = parser.parse_args()
    if args.self_test:
        unittest.main(argv=[sys.argv[0]], exit=True)
    elif args.serve:
        serve()
    else:
        assert args.binary and args.controller_bundle and args.artifacts, 'explicit binary/bundle/evidence required'
        acceptance(args)
