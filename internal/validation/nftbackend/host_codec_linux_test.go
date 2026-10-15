// SPDX-License-Identifier: GPL-3.0-or-later
package nftbackend

import (
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"testing"
)

func TestOriginalDirectionReadbackCodec(t *testing.T) {
	// Linux NFTA_CT_DIRECTION is u8 in a kernel dump, not a big-endian u32.
	data, e := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_CT_KEY, Data: binaryutil.BigEndian.PutUint32(uint32(expr.CtKeyDST))}, {Type: unix.NFTA_CT_DREG, Data: binaryutil.BigEndian.PutUint32(1)}, {Type: unix.NFTA_CT_DIRECTION, Data: []byte{0}}})
	if e != nil {
		t.Fatal(e)
	}
	got := &expr.Ct{}
	if e := expr.Unmarshal(byte(nftables.TableFamilyINet), data, got); e != nil {
		t.Fatalf("kernel original-direction readback lost: %v", e)
	}
}
