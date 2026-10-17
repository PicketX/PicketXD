// SPDX-License-Identifier: GPL-3.0-or-later
package hostrules

import (
	"fmt"
	"github.com/PicketX/PicketXD/internal/markstate"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
)

const strictEstablishedChain = "picketx_strict_established"

// ValidateMarks checks allocation proof in addition to the immutable image.
// Retired assignments remain known; this never reclaims their numeric values.
func (im Image) ValidateMarks(marks markstate.State) error {
	if e := im.Validate(); e != nil {
		return e
	}
	if e := marks.Validate(); e != nil {
		return e
	}
	known := map[uint32]markstate.Assignment{}
	for _, a := range marks.Entries {
		known[a.Mark] = a
	}
	for _, t := range im.Targets {
		a, ok := known[t.Mark]
		if !ok || a.Target != t.Identity {
			return fmt.Errorf("image target absent from durable mark ledger")
		}
	}
	for _, mark := range im.KnownMarks {
		a, ok := known[mark]
		if !ok || a.Target.Key.AgentRef != im.Context.AgentRef || a.Target.Key.ContextID != im.Context.ID {
			return fmt.Errorf("strict known mark absent from durable image scope")
		}
	}
	return nil
}
func conntrackState(mask uint32) []expr.Any {
	return []expr.Any{&expr.Ct{Key: expr.CtKeySTATE, Register: 1}, &expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: binaryutil.NativeEndian.PutUint32(mask), Xor: make([]byte, 4)}, &expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)}}
}
func ownedEstablished(mark uint32) []expr.Any {
	ex := []expr.Any{&expr.Ct{Key: expr.CtKeyMARK, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: binaryutil.NativeEndian.PutUint32(mark)}}
	ex = append(ex, conntrackState(expr.CtStateBitESTABLISHED|expr.CtStateBitRELATED)...)
	return append(ex, &expr.Verdict{Kind: expr.VerdictAccept})
}
