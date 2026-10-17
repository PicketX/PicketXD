// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"errors"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/PicketX/PicketXD/internal/hostrules"
	"net/netip"
	"os"
	"reflect"
	"testing"
)

type cleanupTestKernel struct {
	*applicationKernel
	flows          []hostrevoke.Flow
	dumps, deletes int
	badFamily      bool
	deleteErr      bool
}

func (k *cleanupTestKernel) ConntrackFlows(ctx context.Context, f uint8) ([]hostrevoke.Flow, error) {
	k.dumps++
	if k.badFamily && f == 10 {
		return nil, errors.New("interrupted")
	}
	var out []hostrevoke.Flow
	for _, v := range k.flows {
		if v.Family == f {
			out = append(out, v)
		}
	}
	return out, nil
}
func (k *cleanupTestKernel) DeleteConntrackFlow(ctx context.Context, f hostrevoke.Flow) error {
	k.deletes++
	if k.deleteErr {
		return errors.New("denied")
	}
	for i, v := range k.flows {
		if f == v {
			k.flows = append(k.flows[:i], k.flows[i+1:]...)
			break
		}
	}
	return nil
}
func setupCleanup(t *testing.T) (*Session, *cleanupTestKernel, Config) {
	t.Helper()
	c, e, b := applicationSetup(t)
	k := &cleanupTestKernel{applicationKernel: b}
	e.backend = k
	s := mustOpen(t, c, e)
	return s, k, c
}
func ownedFlow(im hostrules.Image) hostrevoke.Flow {
	var mark uint32
	for _, t := range im.Targets {
		for _, ep := range t.Protected {
			if ep.Protocol == 6 {
				mark = t.Mark
			}
		}
	}
	a, b := netip.MustParseAddr("127.0.0.2"), netip.MustParseAddr("127.0.0.1")
	return hostrevoke.Flow{Family: 2, Mark: mark, ID: [4]byte{1, 2, 3, 4}, Original: hostrevoke.Tuple{Source: a, Destination: b, SourcePort: 33000, DestinationPort: 18443, Protocol: 6}, Reply: hostrevoke.Tuple{Source: b, Destination: a, SourcePort: 18443, DestinationPort: 33000, Protocol: 6}}
}
func TestCleanupUsesConfirmedCurrentAndNeverWritesJournal(t *testing.T) {
	s, k, c := setupCleanup(t)
	defer s.Close()
	ctx := context.Background()
	if _, e := s.RevokeWithdrawn(ctx, nil); e == nil {
		t.Fatal("marker allowed deletion")
	}
	if _, e := s.ApplyPolicy(ctx, admissionPolicy(t, true, false), markRange()); e != nil {
		t.Fatal(e)
	}
	old := *s.record.Application.Confirmed
	k.flows = []hostrevoke.Flow{ownedFlow(old)}
	if _, e := s.ApplyPolicy(ctx, admissionPolicy(t, false, false), markRange()); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(c.StatePath)
	r, e := s.RevokeWithdrawn(ctx, []hostrules.Image{old})
	if e != nil || r.Selected != 1 || r.Deleted != 1 || r.Remaining != 0 || k.deletes != 1 {
		t.Fatal(r, e, k.deletes)
	}
	after, _ := os.ReadFile(c.StatePath)
	if !reflect.DeepEqual(before, after) || !s.ApplicationStatus().EnforcementConfirmed {
		t.Fatal("cleanup changed durable admission or lost proof")
	}
	r, e = s.RevokeWithdrawn(ctx, []hostrules.Image{old})
	if e != nil || r.Selected != 0 || r.Remaining != 0 {
		t.Fatal(r, e)
	}
}
func TestCleanupRefusesInvalidHistoryAndFaultsOnIncompleteEffects(t *testing.T) {
	for _, mode := range []string{"forged mark", "drift", "partial dump", "delete error", "closed", "faulted"} {
		t.Run(mode, func(t *testing.T) {
			s, k, _ := setupCleanup(t)
			defer s.Close()
			ctx := context.Background()
			s.ApplyPolicy(ctx, admissionPolicy(t, true, false), markRange())
			old := *s.record.Application.Confirmed
			k.flows = []hostrevoke.Flow{ownedFlow(old)}
			s.ApplyPolicy(ctx, admissionPolicy(t, false, false), markRange())
			switch mode {
			case "forged mark":
				old.Targets[0].Mark++
			case "drift":
				k.observation.Image = "drift"
			case "partial dump":
				k.badFamily = true
			case "delete error":
				k.deleteErr = true
			case "closed":
				s.Close()
			case "faulted":
				s.faulted = true
			}
			r, e := s.RevokeWithdrawn(ctx, []hostrules.Image{old})
			if e == nil || mode != "delete error" && k.deletes != 0 {
				t.Fatal(mode, r, e, k.deletes)
			}
			if mode != "forged mark" && s.ApplicationStatus().EnforcementConfirmed {
				t.Fatal("cleanup error retained session proof")
			}
		})
	}
	if _, e := (*Session)(nil).RevokeWithdrawn(context.Background(), nil); e == nil {
		t.Fatal("nil session")
	}
}
