package ctbackend

import (
	"context"
	"errors"
	"fmt"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/vishvananda/netlink"
	"sync"
	"time"
)

type Result struct{ Selected, Deleted, Remaining int }
type Backend struct {
	handle *netlink.Handle
	mu     sync.Mutex
	dump   func(netlink.InetFamily) ([]*netlink.ConntrackFlow, error)
	delete func(netlink.InetFamily, []*netlink.ConntrackFlow) (uint, error)
}

func New(h *netlink.Handle) *Backend {
	b := &Backend{handle: h}
	b.dump = func(family netlink.InetFamily) ([]*netlink.ConntrackFlow, error) {
		return h.ConntrackTableList(netlink.ConntrackTable, family)
	}
	b.delete = func(family netlink.InetFamily, flows []*netlink.ConntrackFlow) (uint, error) {
		return h.ConntrackDeleteFilters(netlink.ConntrackTable, family, exactFilter{flows})
	}
	return b
}
func (b *Backend) Revoke(ctx context.Context, before, after fixture.Snapshot, mark uint32, now time.Time) (Result, error) {
	result := Result{Remaining: -1}
	if err := fixture.Validate(before); err != nil {
		return result, err
	}
	if err := fixture.Validate(after); err != nil {
		return result, err
	}
	if mark == 0 {
		return result, fmt.Errorf("ownership mark must be nonzero")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	families := []netlink.InetFamily{netlink.FAMILY_V4, netlink.FAMILY_V6}
	selected := make(map[netlink.InetFamily][]*netlink.ConntrackFlow)
	// Complete both family dumps before the first destructive operation.
	for _, family := range families {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		flows, err := b.dump(family)
		if err != nil {
			return result, fmt.Errorf("initial conntrack dump family %d: %w", family, err)
		}
		selected[family] = SelectWithdrawn(flows, before, after, mark, now)
		result.Selected += len(selected[family])
	}
	var problems []error
	for _, family := range families {
		if err := ctx.Err(); err != nil {
			return result, errors.Join(append(problems, err)...)
		}
		if len(selected[family]) == 0 {
			continue
		}
		count, err := b.delete(family, selected[family])
		result.Deleted += int(count)
		if err != nil {
			problems = append(problems, fmt.Errorf("delete family %d: %w", family, err))
		}
	}
	remaining := 0
	for _, family := range families {
		if err := ctx.Err(); err != nil {
			return result, errors.Join(append(problems, err)...)
		}
		flows, err := b.dump(family)
		if err != nil {
			return result, errors.Join(append(problems, fmt.Errorf("confirmation dump family %d: %w", family, err))...)
		}
		remaining += len(SelectWithdrawn(flows, before, after, mark, now))
	}
	result.Remaining = remaining
	if remaining != 0 {
		problems = append(problems, fmt.Errorf("%d withdrawn owned flows remain", remaining))
	}
	return result, errors.Join(problems...)
}

// Match exact selected original and reply tuples, zone and ownership. Never flush.
type exactFilter struct{ flows []*netlink.ConntrackFlow }

func (f exactFilter) MatchConntrackFlow(flow *netlink.ConntrackFlow) bool {
	if flow == nil {
		return false
	}
	for _, target := range f.flows {
		if flow.Mark == target.Mark && flow.Zone == target.Zone && flow.FamilyType == target.FamilyType && sameTuple(flow.Forward, target.Forward) && sameTuple(flow.Reverse, target.Reverse) {
			return true
		}
	}
	return false
}
func sameTuple(a, b netlink.IPTuple) bool {
	return a.SrcIP.Equal(b.SrcIP) && a.DstIP.Equal(b.DstIP) && a.SrcPort == b.SrcPort && a.DstPort == b.DstPort && a.Protocol == b.Protocol
}
