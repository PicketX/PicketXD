package nftbackend

import (
	"context"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"net/netip"
	"testing"
	"time"
)

func TestApplyRejectsInvalidSnapshotBeforeFlush(t *testing.T) {
	b := New(&nftables.Conn{})
	called := false
	b.batch = func(context.Context, fixture.Snapshot, time.Time) error { called = true; return nil }
	if err := b.Apply(context.Background(), fixture.Snapshot{}, time.Now()); err == nil || called {
		t.Fatalf("invalid snapshot: err=%v mutation=%v", err, called)
	}
}
func TestMembershipUsesRemainingAbsoluteDeadline(t *testing.T) {
	now := time.Unix(1000, 0)
	r := fixture.Rule{Source: netip.MustParsePrefix("192.0.2.0/24"), Endpoint: fixture.Endpoint{Destination: netip.MustParsePrefix("192.0.2.100/32"), Protocol: 6, PortFirst: 8000, PortLast: 8002}, ExpiresAt: now.Add(2400 * time.Millisecond)}
	e, ok := membership(r, now)
	if !ok || e.Timeout != 2400*time.Millisecond || len(e.Key) != 16 || len(e.KeyEnd) != 16 {
		t.Fatalf("element=%+v active=%v", e, ok)
	}
	e, ok = membership(r, now.Add(2*time.Second))
	if !ok || e.Timeout != 400*time.Millisecond {
		t.Fatalf("restart remaining=%v active=%v", e.Timeout, ok)
	}
	if _, ok = membership(r, r.ExpiresAt); ok {
		t.Fatal("expired rule reintroduced")
	}
	if _, ok = membership(r, r.ExpiresAt.Add(-time.Microsecond)); ok {
		t.Fatal("submillisecond deadline became unlimited element")
	}
}
