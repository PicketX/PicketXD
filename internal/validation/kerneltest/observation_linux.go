package kerneltest

import (
	"fmt"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

// ObserveConntrack activates conntrack in the disposable enforcement namespace
// before unfiltered packet proofs. Loading a Netlink handle alone does not
// register that namespace's packet hooks. This rule has no filtering verdict.
func (x *Topology) ObserveConntrack() (func() error, error) {
	table := x.NFT.AddTable(&nftables.Table{Name: "picketx_probe", Family: nftables.TableFamilyINet})
	chain := x.NFT.AddChain(&nftables.Chain{Name: "picketx_observe", Table: table, Type: nftables.ChainTypeFilter, Hooknum: nftables.ChainHookPrerouting, Priority: nftables.ChainPriorityFilter})
	x.NFT.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: []expr.Any{&expr.Ct{Key: expr.CtKeySTATE, Register: 1}, &expr.Counter{}}})
	if e := x.NFT.Flush(); e != nil {
		return nil, fmt.Errorf("enable unfiltered conntrack observation: %w", e)
	}
	return func() error { x.NFT.DelTable(table); return x.NFT.Flush() }, nil
}
