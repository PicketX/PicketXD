// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"encoding/binary"
	nl "github.com/mdlayher/netlink"
	"reflect"
	"testing"
)

func attrs(t *testing.T, a ...nl.Attribute) []byte {
	t.Helper()
	for i := range a {
		a[i].Length = 0
	}
	b, e := nl.MarshalAttributes(a)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func tupleWire(t *testing.T, f Flow, reply bool) []byte {
	t.Helper()
	v := f.Original
	if reply {
		v = f.Reply
	}
	s, d := uint16(1), uint16(2)
	if f.Family == FamilyV6 {
		s, d = 3, 4
	}
	port := func(v uint16) []byte { b := make([]byte, 2); binary.BigEndian.PutUint16(b, v); return b }
	return attrs(t, nl.Attribute{Type: 1 | nl.Nested, Data: attrs(t, nl.Attribute{Type: s, Data: v.Source.AsSlice()}, nl.Attribute{Type: d, Data: v.Destination.AsSlice()})}, nl.Attribute{Type: 2 | nl.Nested, Data: attrs(t, nl.Attribute{Type: 1, Data: []byte{v.Protocol}}, nl.Attribute{Type: 2, Data: port(v.SourcePort)}, nl.Attribute{Type: 3, Data: port(v.DestinationPort)})})
}
func flowWire(t *testing.T, f Flow) nl.Message {
	t.Helper()
	mark := make([]byte, 4)
	binary.BigEndian.PutUint32(mark, f.Mark)
	zone := make([]byte, 2)
	binary.BigEndian.PutUint16(zone, f.Zone)
	return nl.Message{Header: nl.Header{Type: 256}, Data: append([]byte{f.Family, 0, 0, 0}, attrs(t, nl.Attribute{Type: 1 | nl.Nested, Data: tupleWire(t, f, false)}, nl.Attribute{Type: 2 | nl.Nested, Data: tupleWire(t, f, true)}, nl.Attribute{Type: 8, Data: mark}, nl.Attribute{Type: 12, Data: f.ID[:]}, nl.Attribute{Type: 18, Data: zone})...)}
}
func TestDecodeTypedOriginalReplyZoneAndID(t *testing.T) {
	im, _ := testImage(t, rawPolicy(t), nil)
	for _, v6 := range []bool{false, true} {
		for _, proto := range []uint8{6, 17} {
			f := testFlow(im, v6, proto)
			f.ID = [4]byte{0xa1, 0x12, 0x23, 0xb4}
			f.Reply.SourcePort = 9999 // NAT reply can differ from original target.
			msg := flowWire(t, f)
			got, e := Decode(f.Family, []nl.Message{msg})
			if e != nil || !reflect.DeepEqual(got, []Flow{f}) {
				t.Fatal(got, f, e)
			}
			msg.Data[len(msg.Data)-1] ^= 1
			if got[0] != f {
				t.Fatal("wire data aliases flow")
			}
			req, e := DeleteMessage(f)
			if e != nil || req.Header.Type != 258 || req.Header.Flags != nl.Request|nl.Acknowledge || req.Data[0] != f.Family {
				t.Fatal(req, e)
			}
			a, e := nl.UnmarshalAttributes(req.Data[4:])
			if e != nil {
				t.Fatal(e)
			}
			if len(a) != 3 || a[0].Type != 1|nl.Nested || a[1].Type != 18 || a[2].Type != 12 || !reflect.DeepEqual(a[2].Data, f.ID[:]) {
				t.Fatal("delete is not exact original/zone/opaque ID", a)
			}
		}
	}
}
func TestDecodeRejectsIncompleteEvidence(t *testing.T) {
	im, _ := testImage(t, rawPolicy(t), nil)
	for _, mode := range []string{"header", "wrong family", "interrupt", "short attr", "zero attr", "duplicate", "missing id", "short id", "scalar nested", "tuple scalar", "direction zone", "missing reply", "bad protocol", "bad address", "wrong protocol", "bad alignment"} {
		t.Run(mode, func(t *testing.T) {
			f := testFlow(im, false, 6)
			msg := flowWire(t, f)
			a, e := nl.UnmarshalAttributes(msg.Data[4:])
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "header":
				msg.Data = msg.Data[:3]
			case "wrong family":
				msg.Data[0] = 10
			case "interrupt":
				msg.Header.Flags |= nl.DumpInterrupted
			case "short attr":
				msg.Data = append(msg.Data, 1)
			case "zero attr":
				msg.Data = append(msg.Data, 0, 0, 0, 0)
			case "duplicate":
				a = append(a, a[0])
			case "missing id":
				a = append(a[:3], a[4:]...)
			case "short id":
				a[3].Data = []byte{1}
			case "scalar nested":
				a[3].Type |= nl.Nested
			case "tuple scalar":
				a[0].Data = []byte{1, 2}
			case "direction zone":
				a[0].Data = append(a[0].Data, attrs(t, nl.Attribute{Type: 3, Data: []byte{0, 9}})...)
			case "missing reply":
				a = append(a[:1], a[2:]...)
			case "bad protocol":
				f.Reply.Protocol = 1
				a[1].Data = tupleWire(t, f, true)
			case "bad address":
				a[0].Data = attrs(t, nl.Attribute{Type: 1 | nl.Nested, Data: attrs(t, nl.Attribute{Type: 1, Data: []byte{127}}, nl.Attribute{Type: 2, Data: []byte{127, 0, 0, 1}})}, nl.Attribute{Type: 2 | nl.Nested, Data: attrs(t, nl.Attribute{Type: 1, Data: []byte{6}}, nl.Attribute{Type: 2, Data: []byte{1, 2}}, nl.Attribute{Type: 3, Data: []byte{1, 2}})})
			case "wrong protocol":
				msg.Header.Type = 257
			case "bad alignment":
				msg.Data = append(msg.Data, 5, 0, 30, 0, 1)
			}
			if mode == "duplicate" || mode == "missing id" || mode == "short id" || mode == "scalar nested" || mode == "tuple scalar" || mode == "direction zone" || mode == "missing reply" || mode == "bad protocol" || mode == "bad address" {
				msg.Data = append(msg.Data[:4:4], attrs(t, a...)...)
			}
			if got, e := Decode(FamilyV4, []nl.Message{flowWire(t, testFlow(im, false, 6)), msg}); e == nil || len(got) != 0 {
				t.Fatal("partial or malformed evidence accepted", mode, got, e)
			}
		})
	}
	if _, e := DeleteMessage(Flow{}); e == nil {
		t.Fatal("empty flow can flush")
	}
}
func TestDecodeIgnoresUnrelatedProtocolAndMetadata(t *testing.T) {
	im, _ := testImage(t, rawPolicy(t), nil)
	f := testFlow(im, false, 6)
	msg := flowWire(t, f)
	msg.Data = append(msg.Data, attrs(t, nl.Attribute{Type: 42, Data: []byte{1, 2, 3}})...)
	if got, e := Decode(f.Family, []nl.Message{msg}); e != nil || len(got) != 1 {
		t.Fatal(got, e)
	}
	f.Original.Protocol = 1
	f.Reply.Protocol = 1
	msg = flowWire(t, f)
	if got, e := Decode(f.Family, []nl.Message{msg}); e != nil || len(got) != 0 {
		t.Fatal(got, e)
	}
}
