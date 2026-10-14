package ctbackend

import (
	"context"
	"errors"
	"github.com/vishvananda/netlink"
	"testing"
	"time"
)

func TestRevokeReportsInterruptedDump(t *testing.T) {
	b := New(nil)
	deleted := false
	b.dump = func(netlink.InetFamily) ([]*netlink.ConntrackFlow, error) {
		return []*netlink.ConntrackFlow{flow(false)}, netlink.ErrDumpInterrupted
	}
	b.delete = func(netlink.InetFamily, []*netlink.ConntrackFlow) (uint, error) { deleted = true; return 1, nil }
	before := input(false)
	after := input(false)
	after.Allow = nil
	r, err := b.Revoke(context.Background(), before, after, 123, time.Now())
	if !errors.Is(err, netlink.ErrDumpInterrupted) || deleted || r.Remaining != -1 {
		t.Fatalf("result=%+v err=%v delete=%v", r, err, deleted)
	}
}
func TestRevokeReportsPartialDeletion(t *testing.T) {
	b := New(nil)
	b.dump = func(f netlink.InetFamily) ([]*netlink.ConntrackFlow, error) {
		if f == netlink.FAMILY_V4 {
			return []*netlink.ConntrackFlow{flow(false)}, nil
		}
		return nil, nil
	}
	denied := errors.New("delete denied")
	b.delete = func(netlink.InetFamily, []*netlink.ConntrackFlow) (uint, error) { return 0, denied }
	before := input(false)
	after := input(false)
	after.Allow = nil
	r, err := b.Revoke(context.Background(), before, after, 123, time.Now())
	if !errors.Is(err, denied) || r.Selected != 1 || r.Deleted != 0 || r.Remaining != 1 {
		t.Fatalf("partial result=%+v err=%v", r, err)
	}
}
func TestRevokeNeverFlushesTable(t *testing.T) {
	b := New(nil)
	f := flow(false)
	f.Mark = 0
	b.dump = func(netlink.InetFamily) ([]*netlink.ConntrackFlow, error) { return []*netlink.ConntrackFlow{f}, nil }
	deleted := false
	b.delete = func(netlink.InetFamily, []*netlink.ConntrackFlow) (uint, error) { deleted = true; return 1, nil }
	before := input(false)
	after := input(false)
	after.Allow = nil
	r, err := b.Revoke(context.Background(), before, after, 123, time.Now())
	if err != nil || deleted || r.Selected != 0 || r.Deleted != 0 || r.Remaining != 0 {
		t.Fatalf("unowned flow touched: result=%+v err=%v delete=%v", r, err, deleted)
	}
}
