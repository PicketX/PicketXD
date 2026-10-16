// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/binary"
	"errors"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"reflect"
	"testing"
)

func ctMessage(t *testing.T, family uint8, attrs ...nl.Attribute) nl.Message {
	t.Helper()
	data, e := nl.MarshalAttributes(attrs)
	if e != nil {
		t.Fatal(e)
	}
	return nl.Message{Header: nl.Header{Type: nl.HeaderType(ctNewType)}, Data: append([]byte{family, 0, 0, 0}, data...)}
}
func markAttr(mark uint32) nl.Attribute {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, mark)
	return nl.Attribute{Type: ctMarkAttribute, Data: b}
}
func TestRawConntrackMarks(t *testing.T) {
	var families []uint8
	dump := func(_ context.Context, family uint8, consume func([]nl.Message) error) error {
		families = append(families, family)
		return consume([]nl.Message{ctMessage(t, family), ctMessage(t, family, markAttr(0)), ctMessage(t, family, markAttr(99)), ctMessage(t, family, markAttr(100)), ctMessage(t, family, markAttr(107)), ctMessage(t, family, markAttr(108))})
	}
	marks, e := scanConntrackMarks(context.Background(), markRange(), dump)
	if e != nil || !reflect.DeepEqual(marks, []uint32{100, 107}) || !reflect.DeepEqual(families, []uint8{unix.AF_INET, unix.AF_INET6}) {
		t.Fatal("incomplete dual-family occupancy", marks, e, families)
	}
	boom := errors.New("second family interrupted")
	marks, e = scanConntrackMarks(context.Background(), markRange(), func(ctx context.Context, family uint8, consume func([]nl.Message) error) error {
		if family == unix.AF_INET6 {
			return boom
		}
		return dump(ctx, family, consume)
	})
	if !errors.Is(e, boom) || marks != nil {
		t.Fatal("partial occupancy returned", marks, e)
	}
	for _, kind := range []string{"zero-envelope", "trailing-zero-envelope", "short-mark", "nested-mark", "duplicate", "wrong-family", "wrong-type", "short-header", "version", "interrupted", "malformed-envelope"} {
		t.Run(kind, func(t *testing.T) {
			m := ctMessage(t, unix.AF_INET, markAttr(100))
			switch kind {
			case "zero-envelope":
				m = ctMessage(t, unix.AF_INET)
				m.Data = append(m.Data, 0, 0, 1, 0)
			case "trailing-zero-envelope":
				m.Data = append(m.Data, 0, 0, 1, 0)
			case "short-mark":
				m = ctMessage(t, unix.AF_INET, nl.Attribute{Type: ctMarkAttribute, Data: []byte{1}})
			case "nested-mark":
				a := markAttr(100)
				a.Type |= nl.Nested
				m = ctMessage(t, unix.AF_INET, a)
			case "duplicate":
				m = ctMessage(t, unix.AF_INET, markAttr(100), markAttr(101))
			case "wrong-family":
				m.Data[0] = unix.AF_INET6
			case "wrong-type":
				m.Header.Type++
			case "short-header":
				m.Data = m.Data[:3]
			case "version":
				m.Data[1] = 1
			case "interrupted":
				m.Header.Flags = nl.DumpInterrupted
			case "malformed-envelope":
				m.Data = append(m.Data, 1, 2, 3)
			}
			if marks, e := scanConntrackMarks(context.Background(), markRange(), func(_ context.Context, family uint8, consume func([]nl.Message) error) error {
				frame := m
				frame.Data = append([]byte(nil), m.Data...)
				if kind != "wrong-family" && len(frame.Data) >= 4 {
					frame.Data[0] = family
				}
				return consume([]nl.Message{frame})
			}); e == nil || marks != nil {
				t.Fatal("malformed dump accepted", kind, marks, e)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = scanConntrackMarks(ctx, markRange(), dump); e == nil {
		t.Fatal("canceled scan accepted")
	}
}
