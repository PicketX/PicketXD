// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"fmt"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/PicketX/PicketXD/internal/hostrules"
	nl "github.com/mdlayher/netlink"
	"time"
)

type flowKernel interface {
	ConntrackFlows(context.Context, uint8) ([]hostrevoke.Flow, error)
	DeleteConntrackFlow(context.Context, hostrevoke.Flow) error
}

// RevokeWithdrawn cleans only the supplied, validated historical ownership
// scope against this session's actual current image. History is not a receipt
// and callers must retain older images for broader coverage. No journal writes,
// automatic strict scheduling or durable strict-completion proof are implied.
func (s *Session) RevokeWithdrawn(ctx context.Context, history []hostrules.Image) (hostrevoke.Result, error) {
	r := hostrevoke.Result{Remaining: -1}
	if s == nil {
		return r, fmt.Errorf("ownership session required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(ctx); e != nil {
		return r, e
	}
	a := s.record.Application
	if a == nil || a.Confirmed == nil || a.Pending != nil || s.record.Marks == nil {
		return r, fmt.Errorf("confirmed current application and durable mark history required")
	}
	p, e := hostrevoke.New(history, *a.Confirmed, *s.record.Marks)
	if e != nil {
		return r, e
	}
	backend, ok := s.backend.(flowKernel)
	if !ok {
		return r, fmt.Errorf("namespace-bound conntrack backend required")
	}
	r, e = hostrevoke.Run(ctx, p, hostrevoke.Effects{Now: time.Now, Confirm: func(ctx context.Context) error { _, e := s.verifyLocked(ctx); return e }, Dump: backend.ConntrackFlows, Delete: backend.DeleteConntrackFlow})
	if e != nil {
		s.faulted = true
		s.verified = false
		s.applicationVerified = false
	}
	return r, e
}
func (b *nftKernel) ConntrackFlows(ctx context.Context, family uint8) ([]hostrevoke.Flow, error) {
	r, ok := b.reader.(*strictNetlink)
	if !ok {
		return nil, fmt.Errorf("strict namespace-bound conntrack socket required")
	}
	return r.conntrackFlows(ctx, family)
}
func (b *nftKernel) DeleteConntrackFlow(ctx context.Context, f hostrevoke.Flow) error {
	r, ok := b.reader.(*strictNetlink)
	if !ok {
		return fmt.Errorf("strict namespace-bound conntrack socket required")
	}
	m, e := hostrevoke.DeleteMessage(f)
	if e != nil {
		return e
	}
	return r.applyMessages(ctx, []nl.Message{m})
}
func (r *strictNetlink) conntrackFlows(ctx context.Context, family uint8) ([]hostrevoke.Flow, error) {
	var out []hostrevoke.Flow
	e := r.query(ctx, ctGetType, ctNewType, family, nil, maxConntrackDumpBytes, func(messages []nl.Message) error {
		flows, e := hostrevoke.Decode(family, messages)
		if e != nil {
			return e
		}
		if len(flows) > hostrevoke.MaxFlows-len(out) {
			return fmt.Errorf("typed conntrack snapshot exceeds bound")
		}
		out = append(out, flows...)
		return nil
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
