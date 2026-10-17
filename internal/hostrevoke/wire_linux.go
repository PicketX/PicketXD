// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"encoding/binary"
	"fmt"
	nl "github.com/mdlayher/netlink"
	"net/netip"
)

type wireField struct {
	data  []byte
	flags uint16
}

func fields(data []byte) (map[uint16]wireField, error) {
	out := map[uint16]wireField{}
	for len(data) > 0 {
		if len(data) < 4 {
			return nil, fmt.Errorf("truncated conntrack attribute header")
		}
		n := int(binary.NativeEndian.Uint16(data))
		aligned := (n + 3) &^ 3
		if n < 4 || n > len(data) || aligned > len(data) {
			return nil, fmt.Errorf("invalid conntrack attribute envelope")
		}
		kind := binary.NativeEndian.Uint16(data[2:])
		key := kind & 0x3fff
		if _, ok := out[key]; ok {
			return nil, fmt.Errorf("duplicate conntrack attribute %d", key)
		}
		out[key] = wireField{data: data[4:n], flags: kind & 0xc000}
		data = data[aligned:]
	}
	return out, nil
}
func scalar(f map[uint16]wireField, key uint16, n int, required bool) ([]byte, error) {
	v, ok := f[key]
	if !ok && !required {
		return nil, nil
	}
	if !ok || len(v.data) != n || v.flags&nl.Nested != 0 {
		return nil, fmt.Errorf("missing/invalid scalar conntrack attribute %d", key)
	}
	return v.data, nil
}
func nested(f map[uint16]wireField, key uint16) (map[uint16]wireField, error) {
	v, ok := f[key]
	if !ok || v.flags&^uint16(nl.Nested) != 0 {
		return nil, fmt.Errorf("missing/invalid nested conntrack attribute %d", key)
	}
	return fields(v.data)
}
func tuple(f map[uint16]wireField, key uint16, family uint8) (Tuple, error) {
	var t Tuple
	v, e := nested(f, key)
	if e != nil {
		return t, e
	}
	if len(v) != 2 {
		return t, fmt.Errorf("unsupported tuple fields/directional zone")
	}
	ip, e := nested(v, 1)
	if e != nil {
		return t, e
	}
	proto, e := nested(v, 2)
	if e != nil {
		return t, e
	}
	num, e := scalar(proto, 1, 1, true)
	if e != nil {
		return t, e
	}
	t.Protocol = num[0]
	if t.Protocol != 6 && t.Protocol != 17 {
		return t, nil
	}
	if len(proto) != 3 || len(ip) != 2 {
		return t, fmt.Errorf("unsupported TCP/UDP tuple fields")
	}
	src, dst, size := uint16(1), uint16(2), 4
	if family == FamilyV6 {
		src, dst, size = 3, 4, 16
	}
	a, e := scalar(ip, src, size, true)
	if e != nil {
		return t, e
	}
	b, e := scalar(ip, dst, size, true)
	if e != nil {
		return t, e
	}
	t.Source, _ = netip.AddrFromSlice(a)
	t.Destination, _ = netip.AddrFromSlice(b)
	a, e = scalar(proto, 2, 2, true)
	if e != nil {
		return t, e
	}
	b, e = scalar(proto, 3, 2, true)
	if e != nil {
		return t, e
	}
	t.SourcePort = binary.BigEndian.Uint16(a)
	t.DestinationPort = binary.BigEndian.Uint16(b)
	return t, nil
}

// Decode returns no partial evidence on error. Non-TCP/UDP entries are outside
// this engine's scope; unknown top-level metadata remains structurally checked.
func Decode(family uint8, messages []nl.Message) ([]Flow, error) {
	if family != FamilyV4 && family != FamilyV6 {
		return nil, fmt.Errorf("unsupported dump family")
	}
	var out []Flow
	for _, m := range messages {
		if m.Header.Type != 256 || m.Header.Flags&nl.DumpInterrupted != 0 || len(m.Data) < 4 || m.Data[0] != family || m.Data[1] != 0 || m.Data[2] != 0 || m.Data[3] != 0 {
			return nil, fmt.Errorf("unexpected/incomplete conntrack message")
		}
		fs, e := fields(m.Data[4:])
		if e != nil {
			return nil, e
		}
		f := Flow{Family: family}
		f.Original, e = tuple(fs, 1, family)
		if e != nil {
			return nil, e
		}
		f.Reply, e = tuple(fs, 2, family)
		if e != nil {
			return nil, e
		}
		if f.Original.Protocol != f.Reply.Protocol {
			return nil, fmt.Errorf("inconsistent tuple protocols")
		}
		if f.Original.Protocol != 6 && f.Original.Protocol != 17 {
			continue
		}
		id, e := scalar(fs, 12, 4, true)
		if e != nil {
			return nil, e
		}
		copy(f.ID[:], id)
		mark, e := scalar(fs, 8, 4, false)
		if e != nil {
			return nil, e
		}
		if mark != nil {
			f.Mark = binary.BigEndian.Uint32(mark)
		}
		zone, e := scalar(fs, 18, 2, false)
		if e != nil {
			return nil, e
		}
		if zone != nil {
			f.Zone = binary.BigEndian.Uint16(zone)
		}
		if e = f.Validate(); e != nil {
			return nil, e
		}
		out = append(out, f)
		if len(out) > MaxFlows {
			return nil, fmt.Errorf("flow snapshot exceeds bound")
		}
	}
	return out, nil
}
func tupleAttributes(t Tuple, family uint8) ([]byte, error) {
	src, dst := uint16(1), uint16(2)
	if family == FamilyV6 {
		src, dst = 3, 4
	}
	ip, e := nl.MarshalAttributes([]nl.Attribute{{Type: src, Data: t.Source.AsSlice()}, {Type: dst, Data: t.Destination.AsSlice()}})
	if e != nil {
		return nil, e
	}
	be16 := func(n uint16) []byte { b := make([]byte, 2); binary.BigEndian.PutUint16(b, n); return b }
	proto, e := nl.MarshalAttributes([]nl.Attribute{{Type: 1, Data: []byte{t.Protocol}}, {Type: 2, Data: be16(t.SourcePort)}, {Type: 3, Data: be16(t.DestinationPort)}})
	if e != nil {
		return nil, e
	}
	return nl.MarshalAttributes([]nl.Attribute{{Type: 1 | nl.Nested, Data: ip}, {Type: 2 | nl.Nested, Data: proto}})
}

// DeleteMessage always includes an exact original tuple, zone and observed
// opaque kernel ID. Linux checks tuple/zone/ID, not mark or reply fields.
func DeleteMessage(f Flow) (nl.Message, error) {
	if e := f.Validate(); e != nil {
		return nl.Message{}, e
	}
	orig, e := tupleAttributes(f.Original, f.Family)
	if e != nil {
		return nl.Message{}, e
	}
	zone := make([]byte, 2)
	binary.BigEndian.PutUint16(zone, f.Zone)
	data, e := nl.MarshalAttributes([]nl.Attribute{{Type: 1 | nl.Nested, Data: orig}, {Type: 18, Data: zone}, {Type: 12, Data: f.ID[:]}})
	if e != nil {
		return nl.Message{}, e
	}
	return nl.Message{Header: nl.Header{Type: 258, Flags: nl.Request | nl.Acknowledge}, Data: append([]byte{f.Family, 0, 0, 0}, data...)}, nil
}
