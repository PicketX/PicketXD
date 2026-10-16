// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/binary"
	"fmt"
	"syscall"
	"time"

	"github.com/PicketX/PicketXD/internal/hostrules"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

func chainAttributes(c *nftables.Chain) []nl.Attribute {
	a := []nl.Attribute{{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("picketx\x00")}, {Type: unix.NFTA_CHAIN_NAME, Data: []byte(c.Name + "\x00")}}
	if c.Hooknum != nil {
		h, _ := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_HOOK_HOOKNUM, Data: be32(uint32(*c.Hooknum))}, {Type: unix.NFTA_HOOK_PRIORITY, Data: be32(uint32(*c.Priority))}})
		a = append(a, nl.Attribute{Type: unix.NFTA_CHAIN_HOOK | unix.NLA_F_NESTED, Data: h}, nl.Attribute{Type: unix.NFTA_CHAIN_TYPE, Data: []byte(c.Type + "\x00")}, nl.Attribute{Type: unix.NFTA_CHAIN_POLICY, Data: be32(uint32(*c.Policy))})
	}
	return a
}
func be32(n uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, n); return b }
func ruleAttributes(r *nftables.Rule) ([]nl.Attribute, error) {
	var ex []nl.Attribute
	for _, e := range r.Exprs {
		b, err := expr.Marshal(unix.NFPROTO_INET, e)
		if err != nil {
			return nil, err
		}
		ex = append(ex, nl.Attribute{Type: unix.NFTA_LIST_ELEM | unix.NLA_F_NESTED, Data: b})
	}
	b, err := nl.MarshalAttributes(ex)
	if err != nil {
		return nil, err
	}
	a := []nl.Attribute{{Type: unix.NFTA_RULE_TABLE, Data: []byte("picketx\x00")}, {Type: unix.NFTA_RULE_CHAIN, Data: []byte(r.Chain.Name + "\x00")}, {Type: unix.NFTA_RULE_EXPRESSIONS | unix.NLA_F_NESTED, Data: b}}
	if len(r.UserData) > 0 {
		a = append(a, nl.Attribute{Type: unix.NFTA_RULE_USERDATA, Data: r.UserData})
	}
	return a, nil
}

// Serialize native UAPI transactions with Go expression encoders. Deliberately
// omit ECHO: the pinned library cannot decode byteorder expression replies.
func imageMessages(marker string, im hostrules.Image) ([]nl.Message, error) {
	layout, err := im.Layout(marker)
	if err != nil {
		return nil, err
	}
	result := []nl.Message{{Header: nl.Header{Type: nl.HeaderType(unix.NFNL_MSG_BATCH_BEGIN), Flags: nl.Request}, Data: []byte{unix.NFPROTO_UNSPEC, 0, 0, unix.NFNL_SUBSYS_NFTABLES}}}
	add := func(cmd uint16, flags nl.HeaderFlags, a []nl.Attribute) error {
		b, e := nl.MarshalAttributes(a)
		if e != nil {
			return e
		}
		result = append(result, nl.Message{Header: nl.Header{Type: nl.HeaderType(unix.NFNL_SUBSYS_NFTABLES<<8 | cmd), Flags: nl.Request | nl.Acknowledge | flags}, Data: append([]byte{unix.NFPROTO_INET, 0, 0, 0}, b...)})
		return nil
	}
	table := []nl.Attribute{{Type: unix.NFTA_TABLE_NAME, Data: []byte("picketx\x00")}}
	if err = add(unix.NFT_MSG_DELTABLE, 0, table); err != nil {
		return nil, err
	}
	if err = add(unix.NFT_MSG_NEWTABLE, nl.Create|nl.Excl, table); err != nil {
		return nil, err
	}
	for _, c := range layout.Chains {
		if err = add(unix.NFT_MSG_NEWCHAIN, nl.Create|nl.Excl, chainAttributes(c)); err != nil {
			return nil, err
		}
	}
	for _, r := range layout.Rules {
		a, e := ruleAttributes(r)
		if e != nil {
			return nil, e
		}
		if err = add(unix.NFT_MSG_NEWRULE, nl.Create|nl.Append, a); err != nil {
			return nil, err
		}
	}
	result = append(result, nl.Message{Header: nl.Header{Type: nl.HeaderType(unix.NFNL_MSG_BATCH_END), Flags: nl.Request}, Data: []byte{unix.NFPROTO_UNSPEC, 0, 0, unix.NFNL_SUBSYS_NFTABLES}})
	return result, nil
}
func (b *nftKernel) ConfigureImages(marker string, images []hostrules.Image) {
	b.marker = marker
	b.images = images
}
func (b *nftKernel) ApplyImage(ctx context.Context, marker string, im hostrules.Image) error {
	r, ok := b.reader.(*strictNetlink)
	if !ok {
		return fmt.Errorf("namespace-bound native transaction socket required")
	}
	messages, err := imageMessages(marker, im)
	if err != nil {
		return err
	}
	return r.applyMessages(ctx, messages)
}
func (r *strictNetlink) applyMessages(ctx context.Context, messages []nl.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return e
	}
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if e := r.conn.SetDeadline(deadline); e != nil {
		return e
	}
	stop := context.AfterFunc(ctx, func() { r.conn.SetReadDeadline(time.Now()) })
	defer stop()
	sent, err := r.conn.SendMessages(messages)
	if err != nil {
		return err
	}
	pending := map[uint32]nl.Message{}
	for _, m := range sent {
		if m.Header.Flags&nl.Acknowledge != 0 {
			pending[m.Header.Sequence] = m
		}
	}
	buffer := make([]byte, 1<<20)
	for len(pending) > 0 {
		if err = ctx.Err(); err != nil {
			return err
		}
		data, err := r.receiveRaw(buffer)
		if err != nil {
			return err
		}
		raw, err := syscall.ParseNetlinkMessage(data)
		if err != nil {
			return err
		}
		for _, v := range raw {
			m := nl.Message{Header: nl.Header{Type: nl.HeaderType(v.Header.Type), Flags: nl.HeaderFlags(v.Header.Flags), Sequence: v.Header.Seq, PID: v.Header.Pid}, Data: v.Data}
			req, ok := pending[m.Header.Sequence]
			// An error for BATCH_BEGIN also rejects the transaction, even though it
			// does not have an acknowledgement requested.
			if !ok {
				for _, candidate := range sent {
					if candidate.Header.Sequence == m.Header.Sequence {
						req = candidate
						ok = true
						break
					}
				}
			}
			if !ok || m.Header.Type != nl.Error || len(m.Data) < 4 || m.Header.Flags&nl.DumpInterrupted != 0 {
				return fmt.Errorf("unexpected transaction acknowledgement")
			}
			if err = nl.Validate(req, []nl.Message{m}); err != nil {
				return err
			}
			code := int32(binary.NativeEndian.Uint32(m.Data))
			if code != 0 {
				if code < 0 {
					code = -code
				}
				return syscall.Errno(code)
			}
			if _, ok = pending[m.Header.Sequence]; !ok {
				return fmt.Errorf("duplicate or unsolicited acknowledgement")
			}
			delete(pending, m.Header.Sequence)
		}
	}
	return ctx.Err()
}
