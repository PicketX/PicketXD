#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Opt-in real system-manager acceptance, always in a fresh network namespace."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[1]
ENDPOINTS = [(socket.AF_INET, "127.0.0.1"), (socket.AF_INET6, "::1")]


def command(*args, check=True):
    return subprocess.run(args, check=check, text=True, capture_output=True, timeout=30)


def serve():
    def echo(sock, tcp):
        while True:
            if tcp:
                conn, _ = sock.accept()
                with conn:
                    data = conn.recv(64)
                    if data:
                        conn.sendall(data)
            else:
                data, peer = sock.recvfrom(64)
                sock.sendto(data, peer)

    for family, address in ENDPOINTS:
        for tcp, port in [(True, 18443), (False, 18444)]:
            sock = socket.socket(family, socket.SOCK_STREAM if tcp else socket.SOCK_DGRAM)
            if family == socket.AF_INET6:
                sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 1)
            sock.bind((address, port))
            if tcp:
                sock.listen(16)
            threading.Thread(target=echo, args=(sock, tcp), daemon=True).start()
    print("ready", flush=True)
    threading.Event().wait()


def probe(allow):
    for family, address in ENDPOINTS:
        for tcp, port in [(True, 18443), (False, 18444)]:
            echoed = False
            with socket.socket(family, socket.SOCK_STREAM if tcp else socket.SOCK_DGRAM) as sock:
                sock.settimeout(0.6)
                try:
                    sock.connect((address, port))
                    sock.sendall(b"service-probe")
                    echoed = sock.recv(64) == b"service-probe"
                except (TimeoutError, ConnectionError, OSError):
                    pass
            assert echoed == allow, (family, tcp, "expected echo", allow, "observed", echoed)


def acceptance(binary, artifacts):
    assert os.environ.get("PICKETX_SYSTEMD_TESTS") == "1", "Set PICKETX_SYSTEMD_TESTS=1"
    artifacts.mkdir(parents=True, exist_ok=True)
    report = artifacts / "systemd-report.json"
    report.unlink(missing_ok=True)
    (artifacts / "systemd-journal.txt").unlink(missing_ok=True)
    assert os.geteuid() == 0, "Real system-manager validation requires root"
    command("systemctl", "show", "--property=Version")  # Failure is never a skip.
    evidence = {"status": "fail", "scenarios": [], "systemd": command("systemd-analyze", "--version").stdout.strip(),
                "kernel": command("uname", "-r").stdout.strip(),
                "binarySHA256": hashlib.sha256(binary.read_bytes()).hexdigest()}
    work = Path(tempfile.mkdtemp(prefix="picketx-service-", dir="/run"))
    token = work.name
    unit = token + ".service"
    unit_path = Path("/run/systemd/system") / unit
    state = Path("/var/lib") / token
    peer = None
    namespace_created = False
    output = []

    def systemctl(*args, check=True):
        return command("systemctl", *args, check=check)

    def property_value(name):
        return systemctl("show", unit, "--property=" + name, "--value").stdout.strip()

    def statuses(pid):
        journal = command("journalctl", "--unit=" + unit, "--no-pager", "--output=json").stdout
        found = []
        for line in journal.splitlines():
            entry = json.loads(line)
            if entry.get("_PID") != str(pid):
                continue
            try:
                value = json.loads(entry.get("MESSAGE", ""))
            except (ValueError, TypeError):
                continue
            if isinstance(value, dict) and "enforcementConfirmed" in value:
                found.append(value)
        return found

    def wait_status(predicate, old_pid=0):
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            pid = int(property_value("MainPID"))
            if pid and pid != old_pid:
                values = statuses(pid)
                if values and predicate(values[-1]):
                    return pid, values[-1]
            time.sleep(0.2)
        raise AssertionError("No matching live Agent status: " + property_value("ActiveState"))

    def packets(allow):
        command("ip", "netns", "exec", token, sys.executable, str(Path(__file__).resolve()),
                "--probe", "allow" if allow else "deny")

    def publish(content):
        pending = work / "policy.next"
        pending.write_text(content)
        os.replace(pending, work / "policy.yaml")

    try:
        # Exclusive creation and exact recorded cleanup: never borrow a host namespace.
        command("ip", "netns", "add", token)
        namespace_created = True
        command("ip", "-n", token, "link", "set", "lo", "up")
        shutil.copy2(binary, work / "picketxd")
        policy = (ROOT / "examples/enforce/policy.yaml").read_text()
        publish(policy)
        service = (ROOT / "deploy/systemd/picketxd.service").read_text()
        service = service.replace("/usr/local/libexec/picketxd", str(work / "picketxd"))
        service = service.replace("/etc/picketx/policy.yaml", str(work / "policy.yaml"))
        service = service.replace("/var/lib/picketx", str(state)).replace("StateDirectory=picketx", "StateDirectory=" + token)
        service = service.replace("--poll 5s", "--poll 200ms")
        # Only test identity/paths/poll change. All sandbox and restart settings are shipped values.
        service = service.replace("[Service]\n", "[Service]\nNetworkNamespacePath=/run/netns/" + token + "\n")
        unit_path.write_text(service)
        command("systemd-analyze", "verify", "--man=no", "--generators=no", str(unit_path))
        systemctl("daemon-reload")
        peer = subprocess.Popen(["ip", "netns", "exec", token, sys.executable, str(Path(__file__).resolve()), "--serve"],
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        # The parent script is bounded by the CI job timeout; socket probes also prove server readiness.
        import select
        assert select.select([peer.stdout], [], [], 10)[0], "Echo peer did not become ready"
        assert peer.stdout.readline().strip() == "ready", "Echo peer setup failed"
        packets(True)  # Unfiltered baseline and both-family server readiness.
        systemctl("start", unit)
        pid, first = wait_status(lambda st: st["enforcementConfirmed"] and not st["desired"].get("lastError"))
        proc = dict(line.split(":", 1) for line in Path(f"/proc/{pid}/status").read_text().splitlines() if ":" in line)
        assert int(proc["CapEff"], 16) == 1 << 12, proc["CapEff"]  # CAP_NET_ADMIN only.
        assert int(proc["NoNewPrivs"]) == 1
        assert os.stat(f"/proc/{pid}/ns/net").st_ino == os.stat("/run/netns/" + token).st_ino
        assert os.stat(f"/proc/{pid}/ns/net").st_ino != os.stat("/proc/self/ns/net").st_ino
        assert state.stat().st_mode & 0o777 == 0o700
        packets(True)
        evidence["scenarios"].append("sandboxed-start-dual-stack-tcp-udp")
        evidence["effectiveCapabilities"] = proc["CapEff"].strip()
        evidence["noNewPrivileges"] = int(proc["NoNewPrivs"])

        # Actual crash, automatic restart, same identity and durable application/cache recovery.
        systemctl("kill", "--signal=SIGKILL", "--kill-whom=main", unit)
        pid, recovered = wait_status(lambda st: st["enforcementConfirmed"] and st["desired"]["restored"], old_pid=pid)
        assert int(property_value("NRestarts")) >= 1
        assert recovered["desired"]["generation"] == first["desired"]["generation"]
        assert recovered["appliedPolicyHash"] == first["appliedPolicyHash"]
        assert recovered["application"]["imageDigest"] == first["application"]["imageDigest"]
        packets(True)
        evidence["scenarios"].append("sigkill-automatic-restart-retains-identity")

        publish("invalid publication\n")
        _, rejected = wait_status(lambda st: bool(st["desired"].get("lastError")))
        assert rejected["desired"]["generation"] == first["desired"]["generation"]
        assert rejected["appliedPolicyHash"] == first["appliedPolicyHash"]
        packets(True)
        evidence["scenarios"].append("invalid-reload-retains-accepted-policy")

        # Keep Resource/Binding, remove both Grants. Fresh connections must be denied.
        documents = policy.split("---\n")
        publish("---\n".join(d for d in documents if "kind: Grant\n" not in d))
        _, denied = wait_status(lambda st: st["enforcementConfirmed"] and st["desired"]["generation"] > first["desired"]["generation"])
        packets(False)
        evidence["scenarios"].append("atomic-grant-removal-denies-new-dual-stack-flows")
        systemctl("stop", unit)
        assert property_value("MainPID") == "0"
        assert property_value("ActiveState") == "inactive"
        before = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in state.glob("*.json")}
        assert set(before) == {"owner.json", "desired.json"}
        packets(False)
        evidence["scenarios"].append("intentional-stop-retains-filtering-and-state")
        systemctl("start", unit)
        _, final = wait_status(lambda st: st["enforcementConfirmed"] and st["desired"]["restored"])
        assert final["application"]["imageDigest"] == denied["application"]["imageDigest"]
        assert final["desired"]["generation"] == denied["desired"]["generation"]
        assert before == {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in state.glob("*.json")}
        packets(False)
        evidence["scenarios"].append("manual-restart-retains-deny-and-journal-bytes")
        evidence["status"] = "pass"
        evidence["firstStatus"] = first
        evidence["finalStatus"] = final
    finally:
        output.append(command("journalctl", "--unit=" + unit, "--no-pager", "--output=short-precise", check=False).stdout)
        systemctl("stop", unit, check=False)
        if peer is not None:
            peer.terminate()
            try:
                peer.wait(timeout=5)
            except subprocess.TimeoutExpired:
                peer.kill()
                peer.wait(timeout=5)
            output.append(peer.stderr.read())
        if namespace_created:
            command("ip", "netns", "delete", token)
        unit_path.unlink(missing_ok=True)
        systemctl("daemon-reload", check=False)
        systemctl("reset-failed", unit, check=False)
        shutil.rmtree(state, ignore_errors=True)
        shutil.rmtree(work)
        (artifacts / "systemd-journal.txt").write_text("\n".join(output))
        report.write_text(json.dumps(evidence, indent=2) + "\n")
        print(json.dumps(evidence, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serve", action="store_true")
    parser.add_argument("--probe", choices=["allow", "deny"])
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--artifacts", type=Path)
    args = parser.parse_args()
    if args.serve:
        serve()
    elif args.probe:
        probe(args.probe == "allow")
    else:
        assert args.binary and args.artifacts, "--binary and --artifacts required"
        acceptance(args.binary.resolve(), args.artifacts.resolve())
