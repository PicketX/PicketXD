// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/PicketX/PicketXD/internal/markstate"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// Stable ctnetlink UAPI values absent from the pinned x/sys constants.
const ctMarkAttribute uint16 = 8
const ctNewType uint16 = unix.NFNL_SUBSYS_CTNETLINK << 8
const ctGetType uint16 = ctNewType | 1
const maxConntrackDumpBytes = 64 << 20

type ctDumpFunc func(context.Context, uint8, func([]nl.Message) error) error

// scanConntrackMarks keeps only reserved numeric values. Other tuple fields
// remain opaque: this is occupancy, never a flow ownership/deletion proof.
func scanConntrackMarks(ctx context.Context, reservation markstate.Range, dump ctDumpFunc) ([]uint32, error) {
	if err := reservation.Validate(); err != nil {
		return nil, err
	}
	if dump == nil {
		return nil, fmt.Errorf("conntrack dump required")
	}
	seen := make(map[uint32]bool)
	for _, family := range []uint8{unix.AF_INET, unix.AF_INET6} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err := dump(ctx, family, func(messages []nl.Message) error {
			for _, m := range messages {
				if err := ctx.Err(); err != nil {
					return err
				}
				if m.Header.Type != nl.HeaderType(ctNewType) || m.Header.Flags&nl.DumpInterrupted != 0 || len(m.Data) < 4 || m.Data[0] != family || m.Data[1] != 0 {
					return fmt.Errorf("unexpected/incomplete conntrack dump message")
				}
				mark, err := conntrackMark(m.Data[4:])
				if err != nil {
					return err
				}
				if mark >= reservation.First && mark <= reservation.Last {
					seen[mark] = true
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("conntrack mark dump family %d: %w", family, err)
		}
	}
	var marks []uint32
	for mark := range seen {
		marks = append(marks, mark)
	}
	slices.Sort(marks)
	return marks, nil
}
func conntrackMark(data []byte) (uint32, error) {
	if err := validateConntrackEnvelopes(data); err != nil {
		return 0, err
	}
	decoder, err := nl.NewAttributeDecoder(data)
	if err != nil {
		return 0, err
	}
	decoder.ByteOrder = binary.BigEndian
	seen := make(map[uint16]bool)
	var mark uint32
	for decoder.Next() {
		key := decoder.Type()
		if seen[key] {
			return 0, fmt.Errorf("duplicate conntrack attribute %d", key)
		}
		seen[key] = true
		if key == ctMarkAttribute {
			if decoder.TypeFlags()&nl.Nested != 0 {
				return 0, fmt.Errorf("conntrack mark must be scalar")
			}
			mark = decoder.Uint32()
		}
	}
	return mark, decoder.Err()
}

// AttributeDecoder deliberately accepts nla_len=0 for compatibility. Native
// collision evidence requires complete ordinary NLA envelopes, including a
// valid header length and all alignment bytes, even for otherwise opaque fields.
func validateConntrackEnvelopes(data []byte) error {
	for len(data) > 0 {
		if len(data) < 4 {
			return fmt.Errorf("truncated conntrack attribute header")
		}
		length := int(binary.NativeEndian.Uint16(data[:2]))
		if length < 4 || length > len(data) {
			return fmt.Errorf("invalid conntrack attribute envelope length")
		}
		aligned := (length + 3) &^ 3
		if aligned > len(data) {
			return fmt.Errorf("truncated conntrack attribute alignment")
		}
		data = data[aligned:]
	}
	return nil
}

func (r *strictNetlink) OccupiedMarks(ctx context.Context, reservation markstate.Range) ([]uint32, error) {
	return scanConntrackMarks(ctx, reservation, func(ctx context.Context, family uint8, consume func([]nl.Message) error) error {
		return r.query(ctx, ctGetType, ctNewType, family, nil, maxConntrackDumpBytes, consume)
	})
}
