package kerneltest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

var Sources = []netip.Addr{netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("2001:db8:1:0:1::10")}
var Others = []netip.Addr{netip.MustParseAddr("192.0.2.11"), netip.MustParseAddr("2001:db8:1:0:1::11")}
var Destinations = []netip.Addr{netip.MustParseAddr("192.0.2.100"), netip.MustParseAddr("2001:db8:1:0:2::100")}
var NATDestinations = []netip.Addr{netip.MustParseAddr("192.0.2.101"), netip.MustParseAddr("2001:db8:1:0:2::101")}

type Topology struct {
	Enforcement, Client, Server netns.NsHandle
	NFT                         *nftables.Conn
	CT                          *netlink.Handle
	closers                     []io.Closer
}

// withNS changes only a locked thread and restores it before returning.
func withNS(ns netns.NsHandle, fn func() error) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original, err := netns.Get()
	if err != nil {
		return err
	}
	defer original.Close()
	if err = netns.Set(ns); err != nil {
		return err
	}
	defer func() {
		if e := netns.Set(original); e != nil {
			panic(fmt.Sprintf("cannot restore test namespace: %v", e))
		}
	}()
	return fn()
}
func newNS() (ns netns.NsHandle, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original, err := netns.Get()
	if err != nil {
		return -1, err
	}
	defer original.Close()
	ns, err = netns.New()
	if e := netns.Set(original); e != nil {
		panic(fmt.Sprintf("cannot restore namespace: %v", e))
	}
	return ns, err
}
func WithTopology(t *testing.T, run func(*Topology)) {
	t.Helper()
	if os.Getenv("PICKETX_KERNEL_TESTS") != "1" {
		t.Skip("kernel test: explicit opt-in required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Preflight(ctx); err != nil {
		t.Fatal(err)
	}
	x := &Topology{Enforcement: -1, Client: -1, Server: -1}
	t.Cleanup(func() {
		for _, c := range x.closers {
			c.Close()
		}
		if x.NFT != nil {
			x.NFT.CloseLasting()
		}
		if x.CT != nil {
			x.CT.Close()
		}
		for _, n := range []netns.NsHandle{x.Client, x.Server, x.Enforcement} {
			if n.IsOpen() {
				n.Close()
			}
		}
	})
	var err error
	for _, p := range []*netns.NsHandle{&x.Enforcement, &x.Client, &x.Server} {
		if *p, err = newNS(); err != nil {
			t.Fatal(err)
		}
	}
	// All links, routes and sysctls live in our three disposable namespaces.
	if err = x.setup(); err != nil {
		t.Fatal(err)
	}
	if x.NFT, err = nftables.New(nftables.WithNetNSFd(int(x.Enforcement))); err != nil {
		t.Fatal(err)
	}
	if x.CT, err = netlink.NewHandleAt(x.Enforcement, unix.NETLINK_NETFILTER); err != nil {
		t.Fatal(err)
	}
	// Prove both fixture paths work before testing policy. A broken network
	// must never be mistaken for a successful deny.
	for i, src := range Sources {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := x.Probe(ctx, src, Destinations[i], "tcp", 8000)
		cancel()
		if err != nil {
			for _, ns := range []netns.NsHandle{x.Client, x.Enforcement, x.Server} {
				withNS(ns, func() error {
					for _, args := range [][]string{{"-6", "addr"}, {"-6", "route"}, {"-6", "neigh"}} {
						out, e := exec.Command("ip", args...).CombinedOutput()
						t.Logf("namespace %d ip %v: %s (%v)", ns, args, out, e)
					}
					return nil
				})
			}
			t.Fatalf("unfiltered IPv%d fixture failed: %v", src.BitLen(), err)
		}
	}
	run(x)
}
func (x *Topology) setup() error {
	if err := withNS(x.Enforcement, func() error {
		if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
			return err
		}
		if err := os.WriteFile("/proc/sys/net/ipv6/conf/default/forwarding", []byte("1"), 0644); err != nil {
			return err
		}
		if err := os.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte("1"), 0644); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	for i, peer := range []netns.NsHandle{x.Client, x.Server} {
		h, err := netlink.NewHandleAt(x.Enforcement, unix.NETLINK_ROUTE)
		if err != nil {
			return err
		}
		name := fmt.Sprintf("picketx_r%d", i)
		pname := fmt.Sprintf("picketx_p%d", i)
		err = h.LinkAdd(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: name}, PeerName: pname, PeerNamespace: netlink.NsFd(peer)})
		h.Close()
		if err != nil {
			return err
		}
	}
	configs := []struct {
		ns        netns.NsHandle
		name      string
		addresses []string
		gateways  []string
	}{
		{x.Enforcement, "picketx_r0", []string{"192.0.2.1/26", "2001:db8:1:0:1::1/80"}, nil},
		{x.Enforcement, "picketx_r1", []string{"192.0.2.65/26", "2001:db8:1:0:2::1/80"}, nil},
		{x.Client, "picketx_p0", []string{"192.0.2.10/26", "192.0.2.11/26", "2001:db8:1:0:1::10/80", "2001:db8:1:0:1::11/80"}, []string{"192.0.2.1", "2001:db8:1:0:1::1"}},
		{x.Server, "picketx_p1", []string{"192.0.2.100/26", "192.0.2.101/26", "2001:db8:1:0:2::100/80", "2001:db8:1:0:2::101/80"}, []string{"192.0.2.65", "2001:db8:1:0:2::1"}},
	}
	for _, c := range configs {
		h, err := netlink.NewHandleAt(c.ns, unix.NETLINK_ROUTE)
		if err != nil {
			return err
		}
		err = func() error {
			l, e := h.LinkByName(c.name)
			if e != nil {
				return e
			}
			lo, e := h.LinkByName("lo")
			if e != nil {
				return e
			}
			if e = h.LinkSetUp(lo); e != nil {
				return e
			}
			for _, a := range c.addresses {
				addr, e := netlink.ParseAddr(a)
				if e != nil {
					return e
				}
				addr.Flags = unix.IFA_F_NODAD
				if e = h.AddrAdd(l, addr); e != nil {
					return e
				}
			}
			if e = h.LinkSetUp(l); e != nil {
				return e
			}
			for _, g := range c.gateways {
				if e = h.RouteAdd(&netlink.Route{LinkIndex: l.Attrs().Index, Gw: net.ParseIP(g)}); e != nil {
					return e
				}
			}
			return nil
		}()
		h.Close()
		if err != nil {
			return err
		}
	}
	return withNS(x.Server, func() error {
		for _, a := range append(append([]netip.Addr{}, Destinations...), NATDestinations...) {
			for _, port := range []uint16{8000, 8001, 8002, 9000} {
				addr := net.JoinHostPort(a.String(), fmt.Sprint(port))
				ln, err := net.Listen("tcp", addr)
				if err != nil {
					return err
				}
				x.closers = append(x.closers, ln)
				go func() {
					for {
						c, e := ln.Accept()
						if e != nil {
							return
						}
						go func() { defer c.Close(); io.Copy(c, c) }()
					}
				}()
				pc, err := net.ListenPacket("udp", addr)
				if err != nil {
					return err
				}
				x.closers = append(x.closers, pc)
				go func() {
					b := make([]byte, 512)
					for {
						n, p, e := pc.ReadFrom(b)
						if e != nil {
							return
						}
						pc.WriteTo(b[:n], p)
					}
				}()
			}
		}
		return nil
	})
}
func (x *Topology) dial(ctx context.Context, source, destination netip.Addr, protocol string, port uint16) (c net.Conn, err error) {
	err = withNS(x.Client, func() error {
		d := net.Dialer{}
		if protocol == "tcp" {
			d.LocalAddr = &net.TCPAddr{IP: net.IP(source.AsSlice())}
		} else {
			d.LocalAddr = &net.UDPAddr{IP: net.IP(source.AsSlice())}
		}
		var e error
		c, e = d.DialContext(ctx, protocol, net.JoinHostPort(destination.String(), fmt.Sprint(port)))
		return e
	})
	return c, err
}
func (x *Topology) OpenTCP(ctx context.Context, source, destination netip.Addr, port uint16) (net.Conn, error) {
	return x.dial(ctx, source, destination, "tcp", port)
}
func (x *Topology) Probe(ctx context.Context, source, destination netip.Addr, protocol string, port uint16) error {
	c, err := x.dial(ctx, source, destination, protocol, port)
	if err != nil {
		return err
	}
	defer c.Close()
	return Exchange(ctx, c)
}
func Exchange(ctx context.Context, c net.Conn) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(time.Second)
	}
	if err := c.SetDeadline(deadline); err != nil {
		return err
	}
	if _, err := c.Write([]byte("picketx")); err != nil {
		return err
	}
	b := make([]byte, 7)
	if _, err := io.ReadFull(c, b); err != nil {
		return err
	}
	if string(b) != "picketx" {
		return fmt.Errorf("unexpected echo %q", b)
	}
	return nil
}
