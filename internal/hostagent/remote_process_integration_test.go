//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/agentsnapshot"
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type processController struct {
	agentv1.UnimplementedAgentSyncServer
	updates chan *agentv1.Snapshot
	reports chan *agentv1.Report
}

func (c *processController) Connect(st grpc.BidiStreamingServer[agentv1.AgentFrame, agentv1.ControllerFrame]) error {
	md, _ := metadata.FromIncomingContext(st.Context())
	if a := md.Get("authorization"); len(a) != 1 || a[0] != "Bearer kernel-test-token" {
		return fmt.Errorf("bad token")
	}
	hello, e := st.Recv()
	if e != nil {
		return e
	}
	if hello.GetHello().GetProtocolVersion() != 1 {
		return fmt.Errorf("bad hello")
	}
	done := make(chan error, 1)
	go func() {
		for {
			frame, e := st.Recv()
			if e != nil {
				done <- e
				return
			}
			select {
			case c.reports <- frame.GetReport():
			case <-st.Context().Done():
				done <- st.Context().Err()
				return
			}
		}
	}()
	for {
		select {
		case snap := <-c.updates:
			if e := st.Send(&agentv1.ControllerFrame{Snapshot: snap}); e != nil {
				return e
			}
		case e := <-done:
			return e
		case <-st.Context().Done():
			return st.Context().Err()
		}
	}
}
func remoteProcessSetup(t *testing.T, raw string) ([]string, *processController, *grpc.Server) {
	t.Helper()
	root := t.TempDir()
	token := filepath.Join(root, "token")
	if e := os.WriteFile(token, []byte("kernel-test-token"), 0600); e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer()
	controller := &processController{updates: make(chan *agentv1.Snapshot, 8), reports: make(chan *agentv1.Report, 32)}
	agentv1.RegisterAgentSyncServer(server, controller)
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	args := processArgs(t, raw)
	var remote []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--policy" {
			i++
			continue
		}
		remote = append(remote, args[i])
	}
	remote = append(remote, "--controller", listener.Addr().String(), "--agent-token-file", token, "--controller-plaintext")
	return remote, controller, server
}
func remoteSnapshot(t *testing.T, raw string, revision uint64, handling networkplan.HandlingMode) *agentv1.Snapshot {
	t.Helper()
	f := configuration.Facts{AuthorityID: "static-test", Agents: []networkplan.Agent{{Name: "node", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{4, 6}, Handling: []networkplan.HandlingMode{handling}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{{AgentRef: "node", ProfileRef: "host", ID: "remote-request", NetworkSpaceID: "never-local-facts", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: handling, Destinations: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}}}}
	b, e := configuration.Decode([]byte(raw))
	if e != nil {
		t.Fatal(e)
	}
	c, e := configuration.Compile(b, f)
	if e != nil {
		t.Fatal(e)
	}
	snap, e := agentsnapshot.Build(c, "node", revision)
	if e != nil {
		t.Fatal(e)
	}
	return snap
}
func (c *processController) applied(t *testing.T, rev uint64) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case r := <-c.reports:
			if r.Revision == rev && r.Phase == agentv1.Report_APPLIED {
				return
			}
			if r.Revision == rev && r.Phase == agentv1.Report_FAILED {
				t.Fatal("remote apply failed", r)
			}
		case <-timer.C:
			t.Fatal("missing actual applied stream report")
		}
	}
}
func TestEnforceProcessControllerApplyWithdraw(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		raw := example(t)
		args, controller, _ := remoteProcessSetup(t, raw)
		controller.updates <- remoteSnapshot(t, raw, 1, networkplan.Graceful)
		p := startCommand(t, args)
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 1 })
		controller.applied(t, 1)
		processProbe(t, true)
		old := processConnections(t)
		controller.updates <- remoteSnapshot(t, noGrants(raw), 2, networkplan.Graceful)
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 2 })
		controller.applied(t, 2)
		processProbe(t, false)
		processEcho(t, old)
		controller.updates <- remoteSnapshot(t, strings.ReplaceAll(raw, "grant-loopback", "grant-renewed"), 3, networkplan.Graceful)
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 3 })
		controller.applied(t, 3)
		processProbe(t, true)
		controller.updates <- remoteSnapshot(t, emptyPublication(), 4, networkplan.Graceful)
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 4 })
		controller.applied(t, 4)
		processProbe(t, true)
		p.stop(t, syscall.SIGTERM, false)
		t.Log("real Controller TCP/Token -> static Agent binary -> owned host INPUT: four dual-stack TCP/UDP probes allowed; grant removal blocked four fresh probes while old graceful TCP survived; regrant and complete empty withdrawal applied; APPLIED followed native confirmation")
	})
}
func TestEnforceProcessControllerStrictOfflineRecovery(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		expiry := time.Now().Add(4 * time.Second).UTC()
		raw := strings.ReplaceAll(strictExample(t), "validity: {mode: permanent}", "validity: {mode: bounded, expiresAt: '"+expiry.Format(time.RFC3339Nano)+"'}")
		args, controller, server := remoteProcessSetup(t, raw)
		args = changedArg(args, "--handling", "strict")
		controller.updates <- remoteSnapshot(t, raw, 1, networkplan.Strict)
		p := startCommand(t, args)
		first := p.until(t, strictConfirmed)
		controller.applied(t, 1)
		clients := strictProcessConnections(t)
		if !first.Strict.ValidUntil.Equal(expiry) {
			t.Fatal(first)
		}
		server.Stop()
		p.stop(t, syscall.SIGKILL, true)
		time.Sleep(max(0, time.Until(expiry.Add(60*time.Millisecond))))
		for _, c := range clients {
			strictConnectionEcho(t, c, false)
		}
		restarted := startCommand(t, args)
		st := restarted.until(t, strictConfirmed)
		if !st.Desired.Restored || st.Desired.Generation != 1 || !st.Desired.NextChange.IsZero() {
			t.Fatal(st)
		}
		processProbe(t, false)
		for _, c := range clients {
			strictConnectionEcho(t, c, false)
		}
		restarted.stop(t, syscall.SIGTERM, false)
		t.Logf("remote strict revision1 retained absolute deadline %s across Controller outage/SIGKILL; native dual-stack TCP/UDP expired without Agent; offline restart restored provider-bound LKG and confirmed cleanup without extending permission", expiry.Format(time.RFC3339Nano))
	})
}

func TestEnforceProcessRealControllerPublication(t *testing.T) {
	isolatedProcess(t, func() {
		processServers(t)
		root := t.TempDir()
		policy := filepath.Join(root, "policy.yaml")
		factsPath := filepath.Join(root, "facts.json")
		credentialsPath := filepath.Join(root, "credentials.json")
		configPath := filepath.Join(root, "controller.yaml")
		tokenPath := filepath.Join(root, "token")
		raw := example(t)
		publish(t, policy, raw)
		facts := configuration.Facts{AuthorityID: "static-test", Agents: []networkplan.Agent{{Name: "node", Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{4, 6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{{AgentRef: "node", ProfileRef: "host", ID: "controller-context", NetworkSpaceID: "declarative-not-local", Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Handling: networkplan.Graceful, Destinations: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}}}}
		data, e := json.Marshal(facts)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(factsPath, data, 0600); e != nil {
			t.Fatal(e)
		}
		token := "actual-controller-kernel-token"
		if e = os.WriteFile(tokenPath, []byte(token), 0600); e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256([]byte(token))
		credentialData := fmt.Sprintf(`{"credentials":[{"id":"native","agent_ref":"node","sha256":"%x"}]}`, digest)
		if e = os.WriteFile(credentialsPath, []byte(credentialData), 0600); e != nil {
			t.Fatal(e)
		}
		config := fmt.Sprintf("http: {listen: '127.0.0.1:0'}\nagent: {enabled: true, authority: static-test, listen: '127.0.0.1:0'}\nsource: {policy: %q, facts: %q}\ndatabase: %q\ncredentials: %q\npoll: 30ms\n", policy, factsPath, filepath.Join(root, "controller-state", "publication.sqlite"), credentialsPath)
		if e = os.WriteFile(configPath, []byte(config), 0600); e != nil {
			t.Fatal(e)
		}
		binary := os.Getenv("PICKETX_CONTROLLER_BINARY")
		if !filepath.IsAbs(binary) {
			t.Fatal("explicit pinned Core Controller artifact required")
		}
		cmd := exec.Command(binary, "-config", configPath)
		// The Controller correctly rejects unknown runtime keys. Do not leak
		// Agent kernel-harness controls into its production configuration layer.
		harnessKeys := map[string]bool{"PICKETX_KERNEL_TESTS": true, "PICKETX_AGENT_BINARY": true, "PICKETX_CONTROLLER_BINARY": true, "PICKETX_ENFORCE_CASE": true, "PICKETX_TEST_ARTIFACTS": true}
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if !harnessKeys[key] {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		stderr, e := cmd.StderrPipe()
		if e != nil {
			t.Fatal(e)
		}
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		wait := make(chan error, 1)
		go func() { wait <- cmd.Wait() }()
		waited := false
		t.Cleanup(func() {
			if !waited {
				cmd.Process.Kill()
				<-wait
			}
		})
		controllerLog := &lockedLog{}
		scanDone := make(chan struct{})
		started := make(chan string, 1)
		go func() {
			defer close(scanDone)
			scanner := bufio.NewScanner(io.TeeReader(stderr, controllerLog))
			for scanner.Scan() {
				var event struct {
					Message string `json:"msg"`
					Address string `json:"agent_listen"`
				}
				if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Message == "controller started" {
					started <- event.Address
				}
			}
		}()
		var address string
		select {
		case address = <-started:
		case e := <-wait:
			waited = true
			<-scanDone
			t.Fatal("real Controller startup", e, controllerLog.String())
		case <-time.After(5 * time.Second):
			t.Fatal("real Controller startup deadline", controllerLog.String())
		}
		args := processArgs(t, raw)
		var remote []string
		for i := 0; i < len(args); i++ {
			if args[i] == "--policy" {
				i++
				continue
			}
			remote = append(remote, args[i])
		}
		remote = append(remote, "--controller", address, "--agent-token-file", tokenPath, "--controller-plaintext")
		p := startCommand(t, remote)
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 1 })
		processProbe(t, true)
		publish(t, policy, noGrants(raw))
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 2 })
		processProbe(t, false)
		publish(t, policy, emptyPublication())
		p.until(t, func(st Status) bool { return st.EnforcementConfirmed && st.Desired.Generation == 3 })
		processProbe(t, true)
		p.stop(t, syscall.SIGTERM, false)
		if e = cmd.Process.Signal(syscall.SIGTERM); e != nil {
			t.Fatal(e)
		}
		select {
		case e = <-wait:
			waited = true
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("real Controller shutdown deadline")
		}
		t.Log("pinned actual static Core Controller with SQLite authoritative revision1/2/3 and Token enrollment drove actual static Agent: IPv4/IPv6 TCP/UDP allow, grant-only removal default deny, and empty protection withdrawal; distinct remote/local namespace identities mapped safely; SIGTERM lifecycle clean")
	})
}
