//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
)

// Each phase keeps logical identities stable while introducing a distinct source.
func historyPolicy(t *testing.T, phase int) *hostpolicy.Policy {
	t.Helper()
	return cleanupPolicy(t, false, func(in *networkplan.Input) {
		in.Logical.Grants[0].Source = netip.MustParsePrefix("127.0.0.1/32")
		in.Logical.Grants[1].Source = netip.MustParsePrefix("::1/128")
		if phase >= 2 {
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].Revoked = true
			}
			for _, source := range []string{"127.0.0.2/32", "::2/128"} {
				g := in.Logical.Grants[0]
				g.ID = "keep-" + source
				g.Source = netip.MustParsePrefix(source)
				g.Revoked = phase >= 3
				in.Logical.Grants = append(in.Logical.Grants, g)
			}
		}
	})
}
func historyClients(t *testing.T, phase int) []net.Conn {
	t.Helper()
	sources := []string{"127.0.0.1", "::1"}
	if phase == 2 {
		sources = []string{"127.0.0.2", "::2"}
	}
	var clients []net.Conn
	for i, src := range sources {
		dst := []string{"127.0.0.1", "::1"}[i]
		clients = append(clients, cleanupDial(t, "tcp", src, dst, "18443"), cleanupDial(t, "udp", src, dst, "18444"))
	}
	return clients
}
func historyCleanupCheck(t *testing.T, s *Session, c Config, clients []net.Conn) {
	t.Helper()
	st := s.HistoryStatus()
	if !st.Complete || st.Members != 8 || st.Targets != 2 {
		t.Fatal("lost accumulated history", st)
	}
	before, e := os.ReadFile(c.StatePath)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.RevokeRecorded(context.Background())
	if e != nil || r.Selected != 8 || r.Deleted != 8 || r.Remaining != 0 {
		t.Fatal(r, e)
	}
	after, e := os.ReadFile(c.StatePath)
	if e != nil || !reflect.DeepEqual(before, after) || s.HistoryStatus() != st {
		t.Fatal("cleanup altered durable history", e)
	}
	for _, client := range clients {
		cleanupEcho(t, client, false)
	}
	t.Logf("recorded history selected/deleted eight TCP/UDP IPv4/IPv6 flows from two prior source generations; metadata=%+v", st)
}
func TestOwnerKernelHistoryRestartCleanup(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		applyAdmission(t, s, historyPolicy(t, 1))
		clients := historyClients(t, 1)
		applyAdmission(t, s, historyPolicy(t, 2))
		clients = append(clients, historyClients(t, 2)...)
		applyAdmission(t, s, historyPolicy(t, 3))
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		historyCleanupCheck(t, s, c, clients)
	})
}
func TestOwnerKernelHistoryPendingRecovery(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		// Separate subcases use the same owned table after removing the previous
		// confirmed image through the existing namespace-scoped test lifecycle.
		c := kernelConfig(t)
		s := openKernel(t, c)
		applyAdmission(t, s, historyPolicy(t, 1))
		native := s.backend.(*nftKernel)
		s.backend = &rejectedImageKernel{native}
		if _, e := s.ApplyPolicy(context.Background(), historyPolicy(t, 2), kernelMarkRange()); e == nil {
			t.Fatal("malformed batch accepted")
		}
		if s.HistoryStatus().Complete {
			t.Fatal("pending rejection claimed coverage")
		}
		s.Close()
		s = openKernel(t, c)
		if !s.HistoryStatus().Complete || s.HistoryStatus().Members != 4 {
			t.Fatal("rejected candidate remembered", s.HistoryStatus())
		}
		// Real atomic rejection retained phase-one packet admission.
		clients := historyClients(t, 1)
		for _, x := range clients {
			x.Close()
		}
		s.persist = func(r record) error {
			if r.Application != nil && r.Application.Pending == nil && r.Application.Confirmed != nil {
				return errors.New("injected confirmed publication failure")
			}
			return writeRecord(c.StatePath, r)
		}
		if _, e := s.ApplyPolicy(context.Background(), historyPolicy(t, 2), kernelMarkRange()); e == nil {
			t.Fatal("failed publication succeeded")
		}
		if s.HistoryStatus().Complete {
			t.Fatal("pending installation claimed coverage")
		}
		r, _, e := readRecord(c.StatePath)
		if e != nil || r.Application.Pending == nil || len(r.Application.History.State.Targets[0].Members) != 2 {
			t.Fatal("pending history advanced before recovery", e)
		}
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		if !s.HistoryStatus().Complete || s.HistoryStatus().Members != 8 || s.record.Application.Pending != nil {
			t.Fatal("installed candidate not recovered", s.HistoryStatus())
		}
		historyClients(t, 2)
		t.Log("real atomic rejection retained four members; applied pending transaction recovered eight members only after complete native image confirmation")
	})
}
func TestOwnerKernelHistoryLegacyRefusal(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		p := historyPolicy(t, 1)
		first := applyAdmission(t, s, p)
		historyClients(t, 1)
		s.Close()
		r, _, e := readRecord(c.StatePath)
		if e != nil {
			t.Fatal(e)
		}
		r.Version = 3
		r.Application.History = nil
		if e = writeRecord(c.StatePath, r); e != nil {
			t.Fatal(e)
		}
		s = openKernel(t, c)
		before := cleanupSnapshot(t, s.backend.(*nftKernel))
		if _, e = s.RevokeRecorded(context.Background()); e == nil {
			t.Fatal("legacy complete cleanup accepted")
		}
		after := cleanupSnapshot(t, s.backend.(*nftKernel))
		for _, f := range before {
			if !containsFlow(after, f) {
				t.Fatal("legacy refusal deleted a flow", f)
			}
		}
		migrated := applyAdmission(t, s, p)
		if migrated.Attempt != first.Attempt || migrated.ImageDigest != first.ImageDigest || !s.HistoryStatus().Tracked || s.HistoryStatus().Complete {
			t.Fatal("no-op migration changed proof", s.HistoryStatus())
		}
		s.Close()
		s = openKernel(t, c)
		defer s.Close()
		if s.HistoryStatus().Complete {
			t.Fatal("legacy history promoted at restart")
		}
		t.Log("legacy v3 live image retained all four flows; recorded cleanup refused; no-op v4 migration retained attempt/image and incomplete coverage after restart")
	})
}
func TestOwnerKernelHistoryCorruptionRefusal(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		applyAdmission(t, s, historyPolicy(t, 1))
		clients := historyClients(t, 1)
		good, e := os.ReadFile(c.StatePath)
		if e != nil {
			t.Fatal(e)
		}
		s.Close()
		for _, mode := range []string{"missing", "shape", "mark", "endpoint", "lost member"} {
			var r record
			if e = json.Unmarshal(good, &r); e != nil {
				t.Fatal(e)
			}
			if mode == "missing" {
				r.Application.History = nil
			} else {
				target := &r.Application.History.State.Targets[0]
				switch mode {
				case "shape":
					target.Identity.Shape = "invalid"
				case "mark":
					target.Mark++
				case "endpoint":
					target.Members[0].Endpoint.Port++
				case "lost member":
					target.Members = target.Members[1:]
				}
			}
			bad, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(c.StatePath, bad, 0600); e != nil {
				t.Fatal(e)
			}
			opened, e := Open(context.Background(), c)
			if e == nil {
				opened.Close()
				t.Fatal("corrupted history opened", mode)
			}
			after, e := os.ReadFile(c.StatePath)
			if e != nil || !reflect.DeepEqual(bad, after) {
				t.Fatal("rejection rewrote journal", mode, e)
			}
			for _, client := range clients {
				cleanupEcho(t, client, true)
			}
		}
		if e = os.WriteFile(c.StatePath, good, 0600); e != nil {
			t.Fatal(e)
		}
		s = openKernel(t, c)
		defer s.Close()
		if !s.HistoryStatus().Complete {
			t.Fatal(s.HistoryStatus())
		}
		t.Log("five corrupt history forms refused without journal rewrite or loss of four live dual-stack packet flows")
	})
}
func historyHelper(t *testing.T, s *Session) {
	t.Helper()
	scanner := bufio.NewScanner(os.Stdin)
	for phase := 1; phase <= 3; phase++ {
		if phase > 1 && (!scanner.Scan() || scanner.Text() != fmt.Sprint(phase)) {
			t.Fatal("history helper phase input")
		}
		applyAdmission(t, s, historyPolicy(t, phase))
		if e := json.NewEncoder(os.Stdout).Encode(s.HistoryStatus()); e != nil {
			t.Fatal(e)
		}
	}
	// Parent SIGKILLs this process. No normal Close or journal rewrite occurs.
	scanner.Scan()
}
func TestOwnerKernelHistoryProcessRecovery(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		child := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestOwnerKernelHelper$", "-test.v")
		child.Env = append(os.Environ(), "PICKETX_OWNER_HELPER_PATH="+c.StatePath, "PICKETX_OWNER_HELPER_HISTORY=1")
		input, e := child.StdinPipe()
		if e != nil {
			t.Fatal(e)
		}
		output, e := child.StdoutPipe()
		if e != nil {
			t.Fatal(e)
		}
		child.Stderr = os.Stderr
		if e = child.Start(); e != nil {
			t.Fatal(e)
		}
		waited := false
		defer func() {
			input.Close()
			if !waited {
				child.Process.Kill()
				child.Wait()
			}
		}()
		scanner := bufio.NewScanner(output)
		ready := func(want int) {
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "{") {
					continue
				}
				var st HistoryStatus
				if e := json.Unmarshal([]byte(line), &st); e != nil || !st.Complete || st.Members != want {
					t.Fatal("child history", line, e)
				}
				return
			}
			t.Fatal("child did not publish confirmed phase", scanner.Err())
		}
		ready(4)
		clients := historyClients(t, 1)
		if _, e = io.WriteString(input, "2\n"); e != nil {
			t.Fatal(e)
		}
		ready(8)
		clients = append(clients, historyClients(t, 2)...)
		if _, e = io.WriteString(input, "3\n"); e != nil {
			t.Fatal(e)
		}
		ready(8)
		if e = child.Process.Kill(); e != nil {
			t.Fatal(e)
		}
		if e = child.Wait(); e == nil {
			t.Fatal("child was not killed")
		}
		waited = true
		s := openKernel(t, c)
		defer s.Close()
		historyCleanupCheck(t, s, c, clients)
		t.Log("SIGKILL owner released namespace locks; parent's eight established flows survived across three child generations until exact recorded cleanup")
	})
}
