package nftbackend

import (
	"context"
	"errors"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestSlowPreparationRechecksDeadlineAtDispatch(t *testing.T) {
	now := time.Unix(1000, 0)
	end := now.Add(time.Second)
	conn := &nftables.Conn{TestDial: func([]nl.Message) ([]nl.Message, error) { return nil, io.EOF }}
	b := New(conn)
	b.now = func() time.Time { return now }
	b.inject = func(*nftables.Conn) { now = end.Add(time.Millisecond) }
	r := fixture.Rule{Source: mustPrefix("192.0.2.10/32"), Endpoint: fixture.Endpoint{Destination: mustPrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443}, ExpiresAt: end}
	got, e := b.ApplyReceipt(context.Background(), fixture.Snapshot{Protected: []fixture.Endpoint{r.Endpoint}, Allow: []fixture.Rule{r}}, Options{})
	if e != nil || len(got.Emitted.Allow) != 0 || len(got.Omitted.Allow) != 1 || !got.DispatchedAt.Equal(now) {
		t.Fatalf("slow prepare renewed access: emitted=%+v omitted=%+v dispatched=%s err=%v", got.Emitted, got.Omitted, got.DispatchedAt, e)
	}
}

func TestBoundedConfirmationRejectsPermanentNativeElement(t *testing.T) {
	now := time.Unix(1000, 0)
	rule := fixture.Rule{ExpiresAt: now.Add(time.Second)}
	want := setExpectation{rule: rule, element: nftables.SetElement{Key: []byte{1}, KeyEnd: []byte{2}, Timeout: time.Second}, present: true}
	got := nftables.SetElement{Key: []byte{1}, KeyEnd: []byte{2}}
	if e := confirmElements("bounded", want, []nftables.SetElement{got}, now); e == nil {
		t.Fatal("permanent native element confirmed for bounded intent")
	}
	got.Timeout = time.Second
	got.Expires = time.Second
	if e := confirmElements("bounded", want, []nftables.SetElement{got}, now); e != nil {
		t.Fatal(e)
	}
	got.Expires = 0
	if e := confirmElements("bounded", want, []nftables.SetElement{got}, now); e == nil {
		t.Fatal("missing native expiry confirmed far from boundary")
	}
}

func TestReceiptRejectsCallerMutationBeforeReadback(t *testing.T) {
	now := time.Now()
	conn := &nftables.Conn{TestDial: func([]nl.Message) ([]nl.Message, error) { return nil, io.EOF }}
	b := New(conn)
	r := fixture.Rule{Source: mustPrefix("192.0.2.10/32"), Endpoint: fixture.Endpoint{Destination: mustPrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443}, ExpiresAt: now.Add(time.Hour)}
	got, e := b.ApplyReceipt(context.Background(), fixture.Snapshot{Protected: []fixture.Endpoint{r.Endpoint}, Allow: []fixture.Rule{r}}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	got.Emitted.Allow[0].ExpiresAt = time.Time{}
	if e := b.Confirm(context.Background(), got); e == nil || !strings.Contains(e.Error(), "modified receipt") {
		t.Fatal("mutable receipt payload accepted", e)
	}
}

func mustPrefix(s string) netip.Prefix { return netip.MustParsePrefix(s) }
func TestReceiptReportsExpiredAndSubmillisecondOmissions(t *testing.T) {
	now := time.Now()
	r := fixture.Rule{Source: mustPrefix("192.0.2.10/32"), Endpoint: fixture.Endpoint{Destination: mustPrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443}}
	s := fixture.Snapshot{Protected: []fixture.Endpoint{r.Endpoint}, Allow: []fixture.Rule{r, r, r, r}}
	s.Allow[1].ExpiresAt = now.Add(time.Second)
	s.Allow[2].ExpiresAt = now
	s.Allow[3].ExpiresAt = now.Add(time.Microsecond)
	b := New(&nftables.Conn{TestDial: func([]nl.Message) ([]nl.Message, error) { return nil, io.EOF }})
	b.now = func() time.Time { return now }
	receipt, err := b.ApplyReceipt(context.Background(), s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	emitted, omitted := receipt.Emitted, receipt.Omitted
	if len(emitted.Allow) != 2 || len(omitted.Allow) != 2 || !emitted.Allow[1].ExpiresAt.Equal(s.Allow[1].ExpiresAt) {
		t.Fatalf("emitted=%+v omitted=%+v", emitted, omitted)
	}
}
func TestApplyReadsFreshClockAfterLock(t *testing.T) {
	b := New(&nftables.Conn{})
	at := time.Unix(1000, 0)
	b.now = func() time.Time { return at }
	got := time.Time{}
	b.batch = func(_ context.Context, _ fixture.Snapshot, now time.Time) error { got = now; return nil }
	s := fixture.Snapshot{Protected: []fixture.Endpoint{{Destination: mustPrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 443, PortLast: 443}}}
	if e := b.Apply(context.Background(), s, at.Add(-time.Hour)); e != nil || !got.Equal(at) {
		t.Fatalf("dispatch=%v error=%v", got, e)
	}
}
func TestRuleConfirmationIgnoresOnlyCountersAndLookupID(t *testing.T) {
	a := []expr.Any{&expr.Lookup{SourceRegister: 8, SetName: "picketx_allow_0", SetID: 1}, &expr.Counter{}, &expr.Verdict{Kind: expr.VerdictAccept}}
	b := []expr.Any{&expr.Lookup{SourceRegister: 8, SetName: "picketx_allow_0", SetID: 9}, &expr.Counter{Packets: 12, Bytes: 100}, &expr.Verdict{Kind: expr.VerdictAccept}}
	if !sameExpressions(a, b) {
		t.Fatal("counters made semantic mismatch")
	}
	b[2] = &expr.Verdict{Kind: expr.VerdictDrop}
	if sameExpressions(a, b) {
		t.Fatal("different verdict confirmed")
	}
	b[2] = a[2]
	b[0] = &expr.Lookup{SourceRegister: 8, SetName: "foreign_allow"}
	if sameExpressions(a, b) {
		t.Fatal("different permission set confirmed")
	}
}

func TestRuleConfirmationNormalizesKernelRegisterAliases(t *testing.T) {
	actual := []expr.Any{&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}, &expr.Lookup{SourceRegister: 1, SetName: "picketx_allow_0"}, &expr.NAT{Type: expr.NATTypeDestNAT, Family: 2, RegAddrMin: 1, RegProtoMin: 2}}
	expected := []expr.Any{&expr.Payload{DestRegister: 8, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}, &expr.Lookup{SourceRegister: 8, SetName: "picketx_allow_0"}, &expr.NAT{Type: expr.NATTypeDestNAT, Family: 2, RegAddrMin: 8, RegProtoMin: 12}}
	if !sameExpressions(actual, expected) {
		t.Fatal("kernel legacy register alias falsely rejected", expressionDifference(actual, expected))
	}
	expected[0] = &expr.Payload{DestRegister: 9, Base: expr.PayloadBaseNetworkHeader, Offset: 12, Len: 4}
	if sameExpressions(actual, expected) {
		t.Fatal("different register accepted as alias")
	}
}

func TestConntrackWriteDirectionSurvivesReadbackCodec(t *testing.T) {
	want := &expr.Ct{Key: expr.CtKeyMARK, Register: 1, SourceRegister: true}
	raw, e := expr.MarshalExprData(byte(nftables.TableFamilyINet), want)
	if e != nil {
		t.Fatal(e)
	}
	got := &expr.Ct{}
	if e := expr.Unmarshal(byte(nftables.TableFamilyINet), raw, got); e != nil {
		t.Fatal(e)
	}
	if !sameExpressions([]expr.Any{got}, []expr.Any{want}) {
		t.Fatal("dependency decoder lost conntrack write semantics", got, want)
	}
}

func TestFixtureNATDeclaresExactSingleAddressAndPort(t *testing.T) {
	count := 0
	appendFixtureNAT(&nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}, func(c *nftables.Chain) *nftables.Chain { return c }, func(r *nftables.Rule) *nftables.Rule {
		for _, e := range r.Exprs {
			if nat, ok := e.(*expr.NAT); ok {
				count++
				if nat.RegAddrMin == 0 || nat.RegAddrMax != nat.RegAddrMin || nat.RegProtoMin == 0 || nat.RegProtoMax != nat.RegProtoMin || !nat.Specified {
					t.Error("NAT fixture does not explicitly pin single address/port", nat)
				}
			}
		}
		return r
	})
	if count != 2 {
		t.Fatal("missing family NAT fixture", count)
	}
}
func TestMissingReceiptCannotConfirm(t *testing.T) {
	if e := New(&nftables.Conn{}).Confirm(context.Background(), Receipt{}); e == nil {
		t.Fatal("fabricated receipt confirmed")
	}
	if !errors.Is((&ApplyError{Rejected: true, Err: context.Canceled}), context.Canceled) {
		t.Fatal("lost underlying application error")
	}
}
