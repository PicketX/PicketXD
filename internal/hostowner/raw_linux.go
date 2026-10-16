// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

type dumpReader interface {
	Dump(context.Context, uint16, []nl.Attribute) ([]nl.Message, error)
	Close() error
}

// Stable kernel UAPI attributes missing from the pinned x/sys version.
const (
	nftaTableHandle uint16 = 4
	nftaTablePad    uint16 = 5
	nftaChainFlags  uint16 = 10
)

type strictNetlink struct {
	mu   sync.Mutex
	conn *nl.Conn
	raw  syscall.RawConn
}

func newStrictNetlink() (*strictNetlink, error) {
	c, err := nl.Dial(unix.NETLINK_NETFILTER, nil)
	if err != nil {
		return nil, err
	}
	raw, err := c.SyscallConn()
	if err != nil {
		c.Close()
		return nil, err
	}
	return &strictNetlink{conn: c, raw: raw}, nil
}
func (r *strictNetlink) Close() error { return r.conn.Close() }

// Receive raw datagrams before netlink.Conn can discard DONE and its flags.
// The dedicated inspector socket is never read concurrently by Conn.Receive.
func (r *strictNetlink) Dump(ctx context.Context, cmd uint16, attrs []nl.Attribute) ([]nl.Message, error) {
	var result []nl.Message
	err := r.query(ctx, uint16(unix.NFNL_SUBSYS_NFTABLES<<8)|cmd, uint16(unix.NFNL_SUBSYS_NFTABLES<<8)|(cmd-1), unix.NFPROTO_INET, attrs, 4<<20, func(messages []nl.Message) error { result = append(result, messages...); return nil })
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (r *strictNetlink) query(ctx context.Context, request, response uint16, family uint8, attrs []nl.Attribute, maxBytes int, consume func([]nl.Message) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := r.conn.SetDeadline(deadline); err != nil {
		return err
	}
	data, err := nl.MarshalAttributes(attrs)
	if err != nil {
		return err
	}
	req, err := r.conn.Send(nl.Message{Header: nl.Header{Type: nl.HeaderType(request), Flags: nl.Request | nl.Dump}, Data: append([]byte{family, 0, 0, 0}, data...)})
	if err != nil {
		return err
	}
	total := 0
	buffer := make([]byte, 1<<20)
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		var n, flags int
		var from unix.Sockaddr
		var recvErr error
		err = r.raw.Read(func(fd uintptr) bool {
			for {
				n, _, flags, from, recvErr = unix.Recvmsg(int(fd), buffer, nil, 0)
				if errors.Is(recvErr, unix.EINTR) {
					continue
				}
				return !errors.Is(recvErr, unix.EAGAIN) && !errors.Is(recvErr, unix.EWOULDBLOCK)
			}
		})
		if err != nil {
			return err
		}
		if recvErr != nil {
			return recvErr
		}
		peer, ok := from.(*unix.SockaddrNetlink)
		if !ok || peer.Pid != 0 || peer.Groups != 0 || flags&(unix.MSG_TRUNC|unix.MSG_CTRUNC) != 0 || n == 0 {
			return fmt.Errorf("invalid/truncated Netlink datagram")
		}
		total += n
		if total > maxBytes {
			return fmt.Errorf("Netlink dump exceeds bound")
		}
		msgs, done, err := decodeDumpDatagram(buffer[:n], req)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if m.Header.Type != nl.HeaderType(response) || len(m.Data) < 4 || m.Data[0] != family || m.Data[1] != 0 {
				return fmt.Errorf("unexpected Netlink dump message")
			}
		}
		if err = consume(msgs); err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}
func decodeDumpDatagram(data []byte, req nl.Message) ([]nl.Message, bool, error) {
	raw, err := syscall.ParseNetlinkMessage(data)
	if err != nil {
		return nil, false, err
	}
	if len(raw) == 0 {
		return nil, false, fmt.Errorf("empty Netlink datagram")
	}
	var out []nl.Message
	done := false
	for _, v := range raw {
		m := nl.Message{Header: nl.Header{Length: v.Header.Len, Type: nl.HeaderType(v.Header.Type), Flags: nl.HeaderFlags(v.Header.Flags), Sequence: v.Header.Seq, PID: v.Header.Pid}, Data: v.Data}
		if done {
			return nil, false, fmt.Errorf("data after dump completion")
		}
		if m.Header.Flags&nl.DumpInterrupted != 0 {
			return nil, false, fmt.Errorf("interrupted ownership dump")
		}
		if err = nl.Validate(req, []nl.Message{m}); err != nil {
			return nil, false, err
		}
		switch m.Header.Type {
		case nl.Done, nl.Error:
			if len(m.Data) != 0 && len(m.Data) < 4 {
				return nil, false, fmt.Errorf("truncated Netlink status")
			}
			if m.Header.Type == nl.Error && len(m.Data) < 4 {
				return nil, false, fmt.Errorf("missing Netlink error status")
			}
			if len(m.Data) >= 4 {
				code := int32(binary.NativeEndian.Uint32(m.Data))
				if code != 0 {
					if code < 0 {
						code = -code
					}
					return nil, false, syscall.Errno(code)
				}
			}
			done = m.Header.Type == nl.Done
		case nl.Noop:
		default:
			out = append(out, m)
		}
	}
	return out, done, nil
}
func rawFields(data []byte) (map[uint16][]byte, error) {
	d, err := nl.NewAttributeDecoder(data)
	if err != nil {
		return nil, err
	}
	fields := make(map[uint16][]byte)
	for d.Next() {
		key := d.Type()
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("duplicate ownership attribute %d", key)
		}
		fields[key] = d.Bytes()
	}
	return fields, d.Err()
}
func allowedFields(fields map[uint16][]byte, keys ...uint16) error {
	for key := range fields {
		if !slices.Contains(keys, key) {
			return fmt.Errorf("unsupported ownership attribute %d", key)
		}
	}
	return nil
}
func checkRawCounter(data []byte) error {
	d, err := nl.NewAttributeDecoder(data)
	if err != nil {
		return err
	}
	count := 0
	for d.Next() {
		count++
		if count != 1 || d.Type() != unix.NFTA_LIST_ELEM {
			return fmt.Errorf("marker must contain exactly one raw counter expression")
		}
		fields, err := rawFields(d.Bytes())
		if err != nil {
			return err
		}
		if err = allowedFields(fields, unix.NFTA_EXPR_NAME, unix.NFTA_EXPR_DATA); err != nil {
			return err
		}
		body, hasBody := fields[unix.NFTA_EXPR_DATA]
		if string(fields[unix.NFTA_EXPR_NAME]) != "counter\x00" || !hasBody {
			return fmt.Errorf("unsupported raw marker expression")
		}
		counter, err := rawFields(body)
		if err != nil {
			return err
		}
		for key, value := range counter {
			switch key {
			case unix.NFTA_COUNTER_BYTES, unix.NFTA_COUNTER_PACKETS:
				if len(value) != 8 {
					return fmt.Errorf("malformed counter value")
				}
			case unix.NFTA_COUNTER_PAD:
				if len(value) != 0 {
					return fmt.Errorf("malformed counter padding")
				}
			default:
				return fmt.Errorf("unknown counter attribute")
			}
		}
	}
	if err = d.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("missing marker counter")
	}
	return nil
}
func ownedFields(m nl.Message, tableKey uint16) (map[uint16][]byte, bool, error) {
	if len(m.Data) < 4 {
		return nil, false, fmt.Errorf("missing nfgen header")
	}
	fields, err := rawFields(m.Data[4:])
	if err != nil {
		return nil, false, err
	}
	return fields, string(fields[tableKey]) == "picketx\x00", nil
}
func rawTable(ctx context.Context, r dumpReader) (*nftables.Table, error) {
	msgs, err := r.Dump(ctx, unix.NFT_MSG_GETTABLE, nil)
	if err != nil {
		return nil, err
	}
	var tab *nftables.Table
	for _, m := range msgs {
		f, owned, err := ownedFields(m, unix.NFTA_TABLE_NAME)
		if err != nil {
			return nil, err
		}
		if !owned {
			continue
		}
		if tab != nil {
			return nil, fmt.Errorf("duplicate owned table")
		}
		if err = allowedFields(f, unix.NFTA_TABLE_NAME, unix.NFTA_TABLE_FLAGS, unix.NFTA_TABLE_USE, nftaTableHandle, nftaTablePad); err != nil {
			return nil, err
		}
		if len(f[unix.NFTA_TABLE_FLAGS]) != 4 || binary.BigEndian.Uint32(f[unix.NFTA_TABLE_FLAGS]) != 0 {
			return nil, fmt.Errorf("unsupported owned table flags")
		}
		tab = &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
		if use, ok := f[unix.NFTA_TABLE_USE]; ok {
			if len(use) != 4 {
				return nil, fmt.Errorf("malformed table use")
			}
			tab.Use = binary.BigEndian.Uint32(use)
		}
	}
	return tab, nil
}
func strictObserve(ctx context.Context, r dumpReader) (observation, error) {
	tab, err := rawTable(ctx, r)
	if err != nil {
		return observation{}, err
	}
	if tab == nil {
		return observation{}, nil
	}
	attrs := []nl.Attribute{{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("picketx\x00")}}
	msgs, err := r.Dump(ctx, unix.NFT_MSG_GETCHAIN, attrs)
	if err != nil {
		return observation{}, err
	}
	var chains []*nftables.Chain
	for _, m := range msgs {
		f, owned, err := ownedFields(m, unix.NFTA_CHAIN_TABLE)
		if err != nil {
			return observation{}, err
		}
		if !owned {
			continue
		}
		if err = allowedFields(f, unix.NFTA_CHAIN_TABLE, unix.NFTA_CHAIN_NAME, unix.NFTA_CHAIN_HANDLE, unix.NFTA_CHAIN_USE, unix.NFTA_CHAIN_PAD, nftaChainFlags); err != nil {
			return observation{}, err
		}
		if flags, ok := f[nftaChainFlags]; ok && (len(flags) != 4 || binary.BigEndian.Uint32(flags) != 0) {
			return observation{}, fmt.Errorf("unsupported chain flags")
		}
		if string(f[unix.NFTA_CHAIN_NAME]) != ownerChain+"\x00" {
			return observation{}, fmt.Errorf("unrecognized owner chain")
		}
		chains = append(chains, &nftables.Chain{Name: ownerChain, Table: tab})
	}
	if len(chains) != 1 {
		return observation{}, fmt.Errorf("owner chain count mismatch")
	}
	attrs = []nl.Attribute{{Type: unix.NFTA_RULE_TABLE, Data: []byte("picketx\x00")}, {Type: unix.NFTA_RULE_CHAIN, Data: []byte(ownerChain + "\x00")}}
	msgs, err = r.Dump(ctx, unix.NFT_MSG_GETRULE, attrs)
	if err != nil {
		return observation{}, err
	}
	var rules []*nftables.Rule
	for _, m := range msgs {
		f, owned, err := ownedFields(m, unix.NFTA_RULE_TABLE)
		if err != nil {
			return observation{}, err
		}
		if !owned {
			continue
		}
		if err = allowedFields(f, unix.NFTA_RULE_TABLE, unix.NFTA_RULE_CHAIN, unix.NFTA_RULE_HANDLE, unix.NFTA_RULE_EXPRESSIONS, unix.NFTA_RULE_USERDATA, unix.NFTA_RULE_POSITION, unix.NFTA_RULE_PAD); err != nil {
			return observation{}, err
		}
		if string(f[unix.NFTA_RULE_CHAIN]) != ownerChain+"\x00" {
			return observation{}, fmt.Errorf("unexpected rule chain")
		}
		if err = checkRawCounter(f[unix.NFTA_RULE_EXPRESSIONS]); err != nil {
			return observation{}, err
		}
		rules = append(rules, &nftables.Rule{Table: tab, Chain: chains[0], UserData: f[unix.NFTA_RULE_USERDATA], Exprs: []expr.Any{&expr.Counter{}}})
	}
	var counts []int
	for _, cmd := range []uint16{unix.NFT_MSG_GETSET, unix.NFT_MSG_GETOBJ, nftables.NFT_MSG_GETFLOWTABLE} {
		// The table attribute is NLA type 1 for all three object classes.
		objects, err := r.Dump(ctx, cmd, []nl.Attribute{{Type: 1, Data: []byte("picketx\x00")}})
		if err != nil {
			return observation{}, err
		}
		count := 0
		for _, m := range objects {
			_, owned, err := ownedFields(m, 1)
			if err != nil {
				return observation{}, err
			}
			if owned {
				count++
			}
		}
		counts = append(counts, count)
	}
	last, err := rawTable(ctx, r)
	if err != nil {
		return observation{}, err
	}
	if last == nil || *last != *tab {
		return observation{}, fmt.Errorf("owner table changed during inspection")
	}
	return inspectShape(tab, chains, rules, counts[0], counts[1], counts[2])
}
