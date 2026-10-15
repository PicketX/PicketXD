// SPDX-License-Identifier: GPL-3.0-or-later
package nftbackend

import (
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// The host fixture reads original CT addresses/ports at INPUT, after DNAT.
// Family-specific keys are essential: generic SRC/DST read 16 bytes in inet.
func hostEndpointMatch(e fixture.Endpoint) []expr.Any {
	ex := endpointMatch(e)
	key := expr.CtKeyDSTIP
	if e.Destination.Addr().Is6() {
		key = expr.CtKeyDSTIP6
	}
	ex[4] = &expr.Ct{Key: key, Register: 1, Direction: expr.CtDirOriginal}
	ex[7] = &expr.Ct{Key: expr.CtKeyPROTODST, Register: 1, Direction: expr.CtDirOriginal}
	return ex
}
func hostLookup(r fixture.Rule, set *nftables.Set) []expr.Any {
	size := uint32(4)
	src, dst := expr.CtKeySRCIP, expr.CtKeyDSTIP
	if r.Source.Addr().Is6() {
		size = 16
		src, dst = expr.CtKeySRCIP6, expr.CtKeyDSTIP6
	}
	reg := uint32(unix.NFT_REG32_00)
	protoReg := reg + 2*size/4
	portReg := protoReg + 1
	return []expr.Any{
		&expr.Ct{Key: src, Register: reg, Direction: expr.CtDirOriginal},
		&expr.Ct{Key: dst, Register: reg + size/4, Direction: expr.CtDirOriginal},
		&expr.Immediate{Register: protoReg, Data: make([]byte, 4)}, &expr.Ct{Key: expr.CtKeyPROTOCOL, Register: protoReg},
		&expr.Immediate{Register: portReg, Data: make([]byte, 4)}, &expr.Ct{Key: expr.CtKeyPROTODST, Register: portReg, Direction: expr.CtDirOriginal},
		&expr.Lookup{SourceRegister: reg, SetName: set.Name, SetID: set.ID},
	}
}
func untrackedDrop() []expr.Any {
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeySTATE, Register: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: binaryutil.NativeEndian.PutUint32(expr.CtStateBitINVALID | expr.CtStateBitUNTRACKED), Xor: make([]byte, 4)},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)}, &expr.Verdict{Kind: expr.VerdictDrop},
	}
}
