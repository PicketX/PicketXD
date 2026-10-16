// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

const runtimeLockRoot = "/run/picketx"
const ownerChain = "picketx__owner"

type nftKernel struct {
	conn   *nftables.Conn
	reader dumpReader
}

// Open claims a marker-only table. Ownership is not traffic enforcement.
// The fixed lock root prevents a different StatePath from evading exclusion.
func Open(ctx context.Context, c Config) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Create the namespace pin, abstract lock and lasting Netlink socket on
	// this exact OS thread. No setns or CAP_SYS_ADMIN is needed for our own
	// namespace, and later goroutine migration cannot redirect the socket.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pin, err := os.Open("/proc/thread-self/ns/net")
	if err != nil {
		return nil, err
	}
	ns, err := pinnedNamespace(pin)
	if err != nil {
		pin.Close()
		return nil, err
	}
	releaseSocket, err := namespaceSocket()
	if err != nil {
		pin.Close()
		return nil, err
	}
	conn, err := nftables.New(nftables.AsLasting())
	if err != nil {
		releaseSocket()
		pin.Close()
		return nil, err
	}
	reader, err := newStrictNetlink()
	if err != nil {
		conn.CloseLasting()
		releaseSocket()
		pin.Close()
		return nil, err
	}
	return openWith(ctx, c, environment{namespace: ns, lockRoot: runtimeLockRoot, backend: &nftKernel{conn: conn, reader: reader}, release: func() error { return errors.Join(releaseSocket(), pin.Close()) }, revalidate: func() error {
		current, err := pinnedNamespace(pin)
		if err != nil {
			return err
		}
		if current != ns {
			return fmt.Errorf("pinned namespace/boot changed")
		}
		return nil
	}})
}
func pinnedNamespace(f *os.File) (namespace, error) {
	info, err := f.Stat()
	if err != nil {
		return namespace{}, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return namespace{}, fmt.Errorf("namespace stat unavailable")
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return namespace{}, err
	}
	return namespace{BootID: strings.TrimSpace(string(boot)), Device: uint64(st.Dev), Inode: st.Ino}, nil
}
func validMarker(m string) bool {
	p := strings.Split(m, ":")
	return len(p) == 4 && p[0] == "picketx-owner-v1" && hexID(p[1]) && hexID(p[2]) && len(p[3]) == 64 && hexID(p[3][:32]) && hexID(p[3][32:])
}
func inspectShape(tab *nftables.Table, chains []*nftables.Chain, rules []*nftables.Rule, sets, objects, flows int) (observation, error) {
	bad := func() (observation, error) { return observation{}, fmt.Errorf("unsupported ownership table structure") }
	if tab == nil || tab.Name != "picketx" || tab.Family != nftables.TableFamilyINet || tab.Flags != 0 || len(chains) != 1 || sets != 0 || objects != 0 || flows != 0 {
		return bad()
	}
	ch := chains[0]
	if ch == nil || ch.Table == nil || ch.Table.Name != tab.Name || ch.Table.Family != tab.Family || ch.Name != ownerChain || ch.Hooknum != nil || ch.Priority != nil || ch.Policy != nil || ch.Type != "" || ch.Device != "" || len(rules) != 1 {
		return bad()
	}
	rule := rules[0]
	if rule == nil || rule.Table == nil || rule.Table.Name != tab.Name || rule.Table.Family != tab.Family || rule.Chain == nil || rule.Chain.Name != ownerChain || len(rule.Exprs) != 1 || !validMarker(string(rule.UserData)) {
		return bad()
	}
	if _, ok := rule.Exprs[0].(*expr.Counter); !ok {
		return bad()
	}
	return observation{Exists: true, Marker: string(rule.UserData)}, nil
}
func (b *nftKernel) Observe(ctx context.Context) (observation, error) {
	if b.reader == nil {
		return observation{}, fmt.Errorf("strict ownership reader required")
	}
	return strictObserve(ctx, b.reader)
}
func (b *nftKernel) Create(ctx context.Context, m string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validMarker(m) {
		return fmt.Errorf("invalid ownership marker")
	}
	tab := b.conn.CreateTable(&nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet})
	chain := b.conn.AddChain(&nftables.Chain{Name: ownerChain, Table: tab})
	b.conn.AddRule(&nftables.Rule{Table: tab, Chain: chain, UserData: []byte(m), Exprs: []expr.Any{&expr.Counter{}}})
	return b.conn.Flush()
}
func (b *nftKernel) Close() error {
	var err error
	if b.conn != nil {
		err = b.conn.CloseLasting()
	}
	if b.reader != nil {
		err = errors.Join(err, b.reader.Close())
	}
	return err
}
