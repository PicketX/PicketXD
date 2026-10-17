//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PicketX/PicketX/pkg/core"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func strictAdmission(t *testing.T, phase int, edit ...func(*networkplan.Input)) *hostpolicy.Policy {
	t.Helper()
	in, e := historyPolicy(t, phase).Input()
	if e != nil {
		t.Fatal(e)
	}
	for i := range in.Contexts {
		in.Contexts[i].Handling = networkplan.Strict
	}
	for i := range in.Bindings {
		in.Bindings[i].Context.Handling = networkplan.Strict
	}
	for i := range in.Agents {
		in.Agents[i].Capabilities.Handling = append(in.Agents[i].Capabilities.Handling, networkplan.Strict)
	}
	// Context meaning stays immutable; network rows define physical matches.
	bounds := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::/0")}
	in.Contexts[0].Destinations = append([]netip.Prefix{}, bounds...)
	for i := range in.Bindings {
		in.Bindings[i].Context.AllowedDestinations = append([]netip.Prefix{}, bounds...)
	}
	for i := range in.Networks {
		in.Networks[i].Addresses = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
	}
	for _, fn := range edit {
		fn(&in)
	}
	trusted := in.Contexts[0]
	trusted.Destinations = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::/0")}
	scope, e := hostpolicy.NewScope("static", trusted)
	if e != nil {
		t.Fatal(e)
	}
	p, e := hostpolicy.CompileInput(in, scope)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestOwnerKernelStrictMembership(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		allowAll := func(in *networkplan.Input) {
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].Revoked = false
			}
		}
		applyAdmission(t, s, strictAdmission(t, 2, allowAll))
		removed, kept := cleanupClients(t)
		b := s.backend.(*nftKernel)
		for _, family := range []uint8{2, 10} {
			for i, mark := range []uint32{0, 999} {
				f := flowForCleanup(*s.record.Application.Confirmed, family, uint16(38000+i))
				f.Mark = mark
				seedCleanupFlow(t, b, f)
			}
		}
		applyAdmission(t, s, strictAdmission(t, 2, allowAll, func(in *networkplan.Input) {
			for _, src := range []string{"127.0.0.1/32", "::1/128"} {
				in.Logical.Denies = append(in.Logical.Denies, core.Deny{ID: "deny-" + src, Source: netip.MustParsePrefix(src), Scopes: []core.Scope{{ResourceName: "uid"}}, Validity: core.Validity{Mode: core.LifetimePermanent}})
			}
		}))
		for _, client := range removed {
			cleanupEcho(t, client, false)
		}
		for _, client := range kept {
			cleanupEcho(t, client, true)
		}
		plan, e := hostrevoke.NewRecorded(*s.record.Application.History.State, *s.record.Application.Confirmed, *s.record.Marks)
		if e != nil {
			t.Fatal(e)
		}
		before := cleanupSnapshot(t, b)
		var unrelated []hostrevoke.Flow
		for _, f := range before {
			if len(selectedCleanup(t, plan, []hostrevoke.Flow{f})) == 0 {
				unrelated = append(unrelated, f)
			}
		}
		journal, _ := os.ReadFile(c.StatePath)
		r, e := s.RevokeRecorded(context.Background())
		if e != nil || r.Selected != 4 || r.Deleted != 4 || r.Remaining != 0 {
			t.Fatal(r, e)
		}
		after := cleanupSnapshot(t, b)
		for _, f := range unrelated {
			if !containsFlow(after, f) {
				t.Fatal("strict cleanup lost unrelated or independently allowed flow", f)
			}
		}
		for _, client := range kept {
			cleanupEcho(t, client, true)
		}
		current, _ := os.ReadFile(c.StatePath)
		if !reflect.DeepEqual(journal, current) {
			t.Fatal("cleanup wrote journal")
		}
		t.Log("strict deny precedence blocked four owned TCP/UDP IPv4/IPv6 flows before cleanup; exact delete4 remaining0, four independent flows and unmarked/unrelated exact records retained")
	})
}
func TestOwnerKernelStrictPhysicalReplacement(t *testing.T) {
	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprint(i), strictPhysicalReplacement)
	}
}
func strictPhysicalReplacement(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		applyAdmission(t, s, strictAdmission(t, 1))
		clients := historyClients(t, 1)
		// Expand physical matches with fresh-ID reissuance under unchanged context.
		change := func(in *networkplan.Input) {
			for i := range in.Networks {
				in.Networks[i].Addresses = append(in.Networks[i].Addresses, netip.MustParsePrefix("127.0.0.3/32"))
			}
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].ID = "reissued-" + in.Logical.Grants[i].ID
			}
		}
		applyAdmission(t, s, strictAdmission(t, 1, change))
		if len(s.record.Marks.Entries) != 4 || !s.record.Marks.Entries[0].Retired || len(s.record.Application.Confirmed.KnownMarks) != 4 {
			t.Fatal("physical revision did not retain retired aliases", s.record.Marks)
		}
		for _, client := range clients {
			cleanupEcho(t, client, true)
		}
		applyAdmission(t, s, strictAdmission(t, 3, change))
		r, e := s.RevokeRecorded(context.Background())
		if e != nil || r.Selected != 4 || r.Deleted != 4 || r.Remaining != 0 {
			t.Fatal(r, e)
		}
		for _, client := range clients {
			strictPhysicalEchoDenied(t, s, client)
		}
		t.Log("strict physical replacement retained four authorized flows through two retired and two current known marks; later withdrawal selected/deleted4 remaining0")
	})
}
func TestOwnerKernelStrictPendingRecovery(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		applyAdmission(t, s, strictAdmission(t, 1))
		clients := historyClients(t, 1)
		s.backend = &rejectedImageKernel{s.backend.(*nftKernel)}
		if _, e := s.ApplyPolicy(context.Background(), strictAdmission(t, 4), kernelMarkRange()); e == nil {
			t.Fatal("malformed empty replacement accepted")
		}
		s.Close()
		s = openKernel(t, c)
		for _, client := range clients {
			cleanupEcho(t, client, true)
		}
		if r, e := s.RevokeRecorded(context.Background()); e != nil || r.Selected != 0 {
			t.Fatal("recovery lost independent current permission", r, e)
		}
		applyAdmission(t, s, strictAdmission(t, 1))
		s.persist = func(r record) error {
			if r.Application != nil && r.Application.Pending == nil && r.Application.Confirmed != nil {
				return errors.New("confirmed save failed")
			}
			return writeRecord(c.StatePath, r)
		}
		if _, e := s.ApplyPolicy(context.Background(), strictAdmission(t, 3), kernelMarkRange()); e == nil {
			t.Fatal("failed confirmation succeeded")
		}
		for _, client := range clients {
			cleanupEcho(t, client, false)
		}
		if s.HistoryStatus().Complete {
			t.Fatal("pending strict history confirmed")
		}
		s.Close()
		s = openKernel(t, c)
		r, e := s.RevokeRecorded(context.Background())
		if e != nil || r.Selected != 4 || r.Deleted != 4 || r.Remaining != 0 {
			t.Fatal(r, e)
		}
		good, e := os.ReadFile(c.StatePath)
		if e != nil {
			t.Fatal(e)
		}
		s.Close()
		var broken record
		if e = json.Unmarshal(good, &broken); e != nil {
			t.Fatal(e)
		}
		broken.Application.Confirmed.KnownMarks = append(broken.Application.Confirmed.KnownMarks, kernelMarkRange().Last)
		bad, _ := json.Marshal(broken)
		if e = os.WriteFile(c.StatePath, bad, 0600); e != nil {
			t.Fatal(e)
		}
		opened, e := Open(context.Background(), c)
		if e == nil {
			opened.Close()
			t.Fatal("unknown strict alias accepted")
		}
		if !strings.Contains(e.Error(), "known mark absent") {
			t.Fatal("did not reject alias before live observation", e)
		}
		after, _ := os.ReadFile(c.StatePath)
		if !reflect.DeepEqual(after, bad) {
			t.Fatal("alias rejection rewrote journal")
		}
		if e = os.WriteFile(c.StatePath, good, 0600); e != nil {
			t.Fatal(e)
		}
		s = openKernel(t, c)
		defer s.Close()
		t.Log("rejected empty strict batch retained authorized old flows and retirement; installed pending withdrawal blocked packets before publication; reopen confirmed image and deleted4; unknown journal alias refused without rewrite")
	})
}

// Compile the same native inputs without privileges before publishing CI.
func TestStrictNativeInputs(t *testing.T) {
	c, e, b := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	if _, err := s.ApplyPolicy(context.Background(), strictAdmission(t, 1), markRange()); err != nil {
		t.Fatal(err)
	}
	change := func(in *networkplan.Input) {
		for i := range in.Networks {
			in.Networks[i].Addresses = append(in.Networks[i].Addresses, netip.MustParsePrefix("127.0.0.3/32"))
		}
		for i := range in.Logical.Grants {
			in.Logical.Grants[i].ID = "reissued-" + in.Logical.Grants[i].ID
		}
	}
	if _, err := s.ApplyPolicy(context.Background(), strictAdmission(t, 1, change), markRange()); err != nil {
		t.Fatal(err)
	}
	if len(s.record.Marks.Entries) != 4 || len(s.record.Application.Confirmed.KnownMarks) != 4 || b.applies != 2 {
		t.Fatal(s.record.Marks, b.applies)
	}
	if _, err := s.ApplyPolicy(context.Background(), strictAdmission(t, 3, change), markRange()); err != nil {
		t.Fatal(err)
	}
}

func TestStrictDNATInputs(t *testing.T) {
	c, e, _ := applicationSetup(t)
	s := mustOpen(t, c, e)
	defer s.Close()
	apply := func(phase int, prefix string, port uint16) {
		t.Helper()
		p := strictAdmission(t, phase, func(in *networkplan.Input) {
			in.Networks[0].Port = port
			for i := range in.Logical.Grants {
				in.Logical.Grants[i].ID = prefix + in.Logical.Grants[i].ID
			}
		})
		if _, err := s.ApplyPolicy(context.Background(), p, markRange()); err != nil {
			t.Fatal(err)
		}
	}
	apply(3, "", 18443)
	apply(3, "retarget-", 18543)
	apply(1, "grant-", 18543)
	apply(3, "grant-", 18543)
}

// Preserve the failing packet's actual tuples/mark for native diagnosis.
func strictPhysicalEchoDenied(t *testing.T, s *Session, c net.Conn) {
	t.Helper()
	c.SetDeadline(time.Now().Add(250 * time.Millisecond))
	_, e := c.Write([]byte("q"))
	var b [1]byte
	if e == nil {
		_, e = io.ReadFull(c, b[:])
	}
	if e == nil {
		t.Fatalf("strict post-delete packet passed protocol=%s local=%s remote=%s byte=%q current=%+v native=%+v", c.LocalAddr().Network(), c.LocalAddr(), c.RemoteAddr(), b, s.record.Application.Confirmed, cleanupSnapshot(t, s.backend.(*nftKernel)))
	}
	c.SetDeadline(time.Time{})
}

// Conntrack can pick up a connection from the server side after an exact delete.
// Exercise that original/reply orientation explicitly, without flushing the table.
func TestOwnerKernelStrictReverseReconstruction(t *testing.T) {
	isolated(t, func() {
		cleanupServers(t)
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		applyAdmission(t, s, strictAdmission(t, 1))
		clients := historyClients(t, 1)
		b := s.backend.(*nftKernel)
		before := cleanupSnapshot(t, b)
		applyAdmission(t, s, strictAdmission(t, 3))
		r, e := s.RevokeRecorded(context.Background())
		if e != nil || r.Selected != 4 || r.Deleted != 4 || r.Remaining != 0 {
			t.Fatal(r, e)
		}
		seeded := 0
		for _, f := range before {
			if f.Original.Protocol != 17 || f.Original.DestinationPort != 18444 {
				continue
			}
			f.Original, f.Reply = f.Reply, f.Original
			f.Mark = 0
			seedCleanupFlow(t, b, f)
			seeded++
		}
		if seeded != 2 {
			t.Fatal("expected both reversed UDP families", seeded)
		}
		var reversed []hostrevoke.Flow
		for _, f := range cleanupSnapshot(t, b) {
			if f.Mark == 0 && f.Original.Protocol == 17 && f.Original.SourcePort == 18444 {
				reversed = append(reversed, f)
			}
		}
		if len(reversed) != 2 {
			t.Fatal("missing native reversed entries", reversed)
		}
		for _, client := range clients {
			if client.LocalAddr().Network() == "udp" {
				strictPhysicalEchoDenied(t, s, client)
			}
		}
		if r, e := s.RevokeRecorded(context.Background()); e != nil || r.Selected != 0 {
			t.Fatal("unmarked reversed entries must not be deleted", r, e)
		}
		after := cleanupSnapshot(t, b)
		for _, f := range reversed {
			if !containsFlow(after, f) {
				t.Fatal("unmarked reversed record lost", f)
			}
		}
		t.Log("both reversed unmarked UDP entries could not bypass strict withdrawal and were preserved by cleanup")
	})
}

func TestOwnerKernelStrictOriginalDNAT(t *testing.T) {
	isolated(t, func() {
		admissionServers(t)
		n := connectKernel(t)
		tab := n.CreateTable(&nftables.Table{Name: "picketx_test_nat", Family: nftables.TableFamilyINet})
		priority := nftables.ChainPriority(-100)
		ch := n.AddChain(&nftables.Chain{Name: "picketx_test_output", Table: tab, Type: nftables.ChainTypeNAT, Hooknum: nftables.ChainHookOutput, Priority: &priority})
		for _, family := range []byte{unix.NFPROTO_IPV4, unix.NFPROTO_IPV6} {
			n.AddRule(&nftables.Rule{Table: tab, Chain: ch, Exprs: []expr.Any{&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{family}}, &expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}}, &expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 2, Len: 2}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x48, 0x6f}}, &expr.Immediate{Register: 1, Data: []byte{0x48, 0x0b}}, &expr.NAT{Type: expr.NATTypeDestNAT, Family: uint32(family), RegProtoMin: 1, RegProtoMax: 1}}})
		}
		if e := n.Flush(); e != nil {
			t.Fatal(e)
		}
		c := kernelConfig(t)
		s := openKernel(t, c)
		defer s.Close()
		// An original destination outside scope remains outside scope after DNAT.
		applyAdmission(t, s, strictAdmission(t, 3))
		for _, address := range []string{"127.0.0.1:18543", "[::1]:18543"} {
			client := admissionConn(t, address, true)
			client.Close()
		}
		edit := func(prefix string) func(*networkplan.Input) {
			return func(in *networkplan.Input) {
				in.Networks[0].Port = 18543
				for i := range in.Logical.Grants {
					in.Logical.Grants[i].ID = prefix + in.Logical.Grants[i].ID
				}
			}
		}
		applyAdmission(t, s, strictAdmission(t, 3, edit("retarget-")))
		for _, address := range []string{"127.0.0.1:18543", "[::1]:18543"} {
			admissionConn(t, address, false)
		}
		applyAdmission(t, s, strictAdmission(t, 1, edit("grant-")))
		var clients []net.Conn
		for _, address := range []string{"127.0.0.1:18543", "[::1]:18543"} {
			clients = append(clients, admissionConn(t, address, true))
		}
		applyAdmission(t, s, strictAdmission(t, 3, edit("grant-")))
		r, e := s.RevokeRecorded(context.Background())
		if e != nil || r.Selected != 2 || r.Deleted != 2 || r.Remaining != 0 {
			t.Fatal(r, e)
		}
		for _, client := range clients {
			strictPhysicalEchoDenied(t, s, client)
			client.Close()
		}
		t.Log("strict preserved outside-scope original DNAT alias, enforced original protected DNAT port in both families, and exact-deleted two withdrawn TCP flows")
	})
}
