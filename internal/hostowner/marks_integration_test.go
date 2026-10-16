//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketXD/internal/markstate"
	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func kernelMarkRange() markstate.Range { return markstate.Range{First: 0x50490000, Last: 0x50490007} }
func reserveKernelMarks(t *testing.T, s *Session, targets ...markstate.Target) MarkStatus {
	t.Helper()
	st, err := s.ReconcileMarks(context.Background(), kernelMarkRange(), targets)
	if err != nil || st.ObservedAt.IsZero() {
		t.Fatal("native reservation failed", st, err)
	}
	return st
}
func activeMark(t *testing.T, st MarkStatus, id string) uint32 {
	t.Helper()
	for _, a := range st.State.Entries {
		if !a.Retired && a.Target.Key.Entry.ResourceName == id {
			return a.Mark
		}
	}
	t.Fatal("missing active assignment", id, st)
	return 0
}
func ctHandle(t *testing.T) *netlink.Handle {
	t.Helper()
	h, err := netlink.NewHandle(unix.NETLINK_NETFILTER)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h
}
func createMarkedFlow(t *testing.T, h *netlink.Handle, family uint8, mark uint32, port uint16) *netlink.ConntrackFlow {
	t.Helper()
	source, destination := net.ParseIP("192.0.2.10").To4(), net.ParseIP("192.0.2.100").To4()
	if family == unix.AF_INET6 {
		source, destination = net.ParseIP("fd66::1"), net.ParseIP("fd66::2")
	}
	flow := &netlink.ConntrackFlow{FamilyType: family, Mark: mark, TimeOut: 120, Forward: netlink.IPTuple{SrcIP: source, DstIP: destination, SrcPort: port, DstPort: 8443, Protocol: 17}, Reverse: netlink.IPTuple{SrcIP: destination, DstIP: source, SrcPort: 8443, DstPort: port, Protocol: 17}}
	if err := h.ConntrackCreate(netlink.ConntrackTable, netlink.InetFamily(family), flow); err != nil {
		t.Fatal("native CT fixture insertion failed", err)
	}
	assertFlowPresent(t, h, flow)
	return flow
}

type insertedFlowFilter struct{ flow *netlink.ConntrackFlow }

func (f insertedFlowFilter) MatchConntrackFlow(v *netlink.ConntrackFlow) bool {
	a := f.flow
	tuple := func(x, y netlink.IPTuple) bool {
		return x.SrcIP.Equal(y.SrcIP) && x.DstIP.Equal(y.DstIP) && x.SrcPort == y.SrcPort && x.DstPort == y.DstPort && x.Protocol == y.Protocol
	}
	return v != nil && v.FamilyType == a.FamilyType && v.Mark == a.Mark && v.Zone == a.Zone && tuple(v.Forward, a.Forward) && tuple(v.Reverse, a.Reverse)
}
func assertFlowPresent(t *testing.T, h *netlink.Handle, flow *netlink.ConntrackFlow) {
	t.Helper()
	flows, err := h.ConntrackTableList(netlink.ConntrackTable, netlink.InetFamily(flow.FamilyType))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range flows {
		if (insertedFlowFilter{flow}).MatchConntrackFlow(v) {
			return
		}
	}
	t.Fatal("independent readback did not find inserted flow", flow)
}
func deleteInsertedFlow(t *testing.T, h *netlink.Handle, flow *netlink.ConntrackFlow) {
	t.Helper()
	count, err := h.ConntrackDeleteFilters(netlink.ConntrackTable, netlink.InetFamily(flow.FamilyType), insertedFlowFilter{flow})
	if err != nil || count != 1 {
		t.Fatal("exact fixture teardown failed", count, err)
	}
}

func TestOwnerKernelMarkLifecycle(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		s := openKernel(t, c)
		first := reserveKernelMarks(t, s, markTarget("a"), markTarget("b"))
		old := activeMark(t, first, "a")
		h := ctHandle(t)
		flow := createMarkedFlow(t, h, unix.AF_INET, old, 40001)
		reserveKernelMarks(t, s, markTarget("b"))
		next := reserveKernelMarks(t, s, markTarget("a"), markTarget("b"))
		if activeMark(t, next, "a") == old || len(next.State.Entries) != 3 || !next.State.Entries[0].Retired {
			t.Fatal("live retired mark reused", next)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s = openKernel(t, c)
		defer s.Close()
		restored := reserveKernelMarks(t, s, markTarget("b"), markTarget("a"))
		if !reflect.DeepEqual(restored.State, next.State) || s.Status().EnforcementConfirmed {
			t.Fatal("native restart changed history/status", restored)
		}
		assertFlowPresent(t, h, flow)
		t.Logf("native v2 restart retained old flow mark=%#x; recreated target mark=%#x; enforcement=false", old, activeMark(t, restored, "a"))
	})
}
func TestOwnerKernelMarkCollision(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		h := ctHandle(t)
		r := kernelMarkRange()
		v4 := createMarkedFlow(t, h, unix.AF_INET, r.Last, 40002)
		v6 := createMarkedFlow(t, h, unix.AF_INET6, r.Last-1, 40003)
		control := createMarkedFlow(t, h, unix.AF_INET6, 0x77770001, 40004)
		marks, err := s.backend.(markObserver).OccupiedMarks(context.Background(), r)
		if err != nil || !reflect.DeepEqual(marks, []uint32{r.Last - 1, r.Last}) {
			t.Fatal("strict dual-family occupancy missing", marks, err)
		}
		before, _ := os.ReadFile(c.StatePath)
		if _, err = s.ReconcileMarks(context.Background(), r, []markstate.Target{markTarget("a")}); err == nil {
			t.Fatal("far reserved collisions accepted")
		}
		after, _ := os.ReadFile(c.StatePath)
		if !bytes.Equal(before, after) {
			t.Fatal("collision published ledger")
		}
		deleteInsertedFlow(t, h, v4)
		if _, err = s.ReconcileMarks(context.Background(), r, []markstate.Target{markTarget("a")}); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%#x", r.Last-1)) {
			t.Fatal("IPv6-only collision missed", err)
		}
		deleteInsertedFlow(t, h, v6)
		st := reserveKernelMarks(t, s, markTarget("a"))
		if activeMark(t, st, "a") != r.First {
			t.Fatal("failed attempt consumed marks", st)
		}
		assertFlowPresent(t, h, control)
		t.Logf("full v4/v6 scan found far-end reserved marks %#x/%#x; unrelated marked flow preserved", r.Last, r.Last-1)
	})
}

type markHelperReady struct {
	Owner Status
	Marks MarkStatus
}

func TestOwnerKernelMarkProcessRecovery(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		child := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestOwnerKernelHelper$", "-test.v")
		child.Env = append(os.Environ(), "PICKETX_OWNER_HELPER_PATH="+c.StatePath, "PICKETX_OWNER_HELPER_MARKS=1")
		stdout, err := child.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdin, err := child.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer stdin.Close()
		child.Stderr = os.Stderr
		if err = child.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if child.ProcessState == nil {
				child.Process.Kill()
				child.Wait()
			}
		}()
		reader := bufio.NewReader(stdout)
		var ready markHelperReady
		for {
			line, e := reader.ReadBytes('\n')
			if e != nil {
				t.Fatal("mark helper exited before readiness", string(line), e)
			}
			if json.Unmarshal(line, &ready) == nil && ready.Owner.OwnershipConfirmed && len(ready.Marks.State.Entries) == 1 {
				break
			}
		}
		old := activeMark(t, ready.Marks, "a")
		if err = child.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		if child.Wait() == nil {
			t.Fatal("helper was not killed")
		}
		io.Copy(io.Discard, reader)
		s := openKernel(t, c)
		defer s.Close()
		st := reserveKernelMarks(t, s, markTarget("a"))
		if activeMark(t, st, "a") != old || s.Status().OwnerID != ready.Owner.OwnerID {
			t.Fatal("killed process lost durable allocation", st)
		}
		reserveKernelMarks(t, s)
		st = reserveKernelMarks(t, s, markTarget("a"))
		if activeMark(t, st, "a") == old {
			t.Fatal("killed-process recovery reused retired mark")
		}
		t.Logf("standalone helper killed without Close: allocation %#x recovered, retired, then replaced by %#x", old, activeMark(t, st, "a"))
	})
}
func TestOwnerKernelMarkDrift(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		reserveKernelMarks(t, s, markTarget("a"))
		before, _ := os.ReadFile(c.StatePath)
		n := connectKernel(t)
		n.AddChain(&nftables.Chain{Name: "picketx_extra", Table: kernelTable()})
		if err := n.Flush(); err != nil {
			t.Fatal(err)
		}
		if st, err := s.ReconcileMarks(context.Background(), kernelMarkRange(), []markstate.Target{markTarget("b")}); err == nil || len(st.State.Entries) != 0 {
			t.Fatal("drift allocated marks", st, err)
		}
		after, _ := os.ReadFile(c.StatePath)
		if !bytes.Equal(before, after) {
			t.Fatal("drift changed durable ledger")
		}
		t.Log("unexpected live chain rejected before mark publication")
	})
}
