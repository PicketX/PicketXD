//go:build integration

// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PicketX/PicketXD/internal/markstate"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

func isolated(t *testing.T, run func()) {
	t.Helper()
	if os.Getenv("PICKETX_KERNEL_TESTS") != "1" {
		t.Skip("explicit kernel opt-in required")
	}
	if os.Getenv("PICKETX_OWNER_CASE") == t.Name() {
		run()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "unshare", "--net", os.Args[0], "-test.run", "^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), "PICKETX_OWNER_CASE="+t.Name())
	data, err := cmd.CombinedOutput()
	t.Logf("isolated process:\n%s", data)
	if err != nil {
		t.Fatalf("isolated owner case failed: %v", err)
	}
}
func kernelConfig(t *testing.T) Config {
	t.Helper()
	return Config{StatePath: filepath.Join(t.TempDir(), "state", "owner.json"), ProviderID: "static", AgentRef: "node", RuntimeKey: "host-scope"}
}
func openKernel(t *testing.T, c Config) *Session {
	t.Helper()
	s, err := Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func connectKernel(t *testing.T) *nftables.Conn {
	t.Helper()
	c, err := nftables.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseLasting() })
	return c
}
func kernelTable() *nftables.Table {
	return &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
}

func TestOwnerKernelSocketExclusion(t *testing.T) {
	isolated(t, func() {
		release, err := namespaceSocket()
		if err != nil {
			t.Fatal(err)
		}
		second, err := namespaceSocket()
		if err == nil {
			second()
			release()
			t.Fatal("duplicate namespace singleton accepted")
		}
		if err = release(); err != nil {
			t.Fatal(err)
		}
		release, err = namespaceSocket()
		if err != nil {
			t.Fatal("namespace lock not released", err)
		}
		defer release()
	})
}

func TestOwnerKernelFreshAndRestart(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		s := openKernel(t, c)
		first := s.Status()
		if !first.OwnershipConfirmed || first.EnforcementConfirmed {
			t.Fatalf("invalid status %+v", first)
		}
		info, err := os.Stat(c.StatePath)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Close(); err != nil {
			t.Fatal(err)
		}
		s = openKernel(t, c)
		defer s.Close()
		st, err := s.Verify(context.Background())
		if err != nil || st.OwnerID != first.OwnerID || st.Epoch != first.Epoch || !st.OwnershipConfirmed {
			t.Fatalf("restart status %+v %v", st, err)
		}
		now, _ := os.Stat(c.StatePath)
		if now.ModTime() != info.ModTime() {
			t.Fatal("confirmed restart rewrote journal")
		}
		t.Logf("same namespace real marker reobserved owner=%s epoch=%s enforcement=false", st.OwnerID, st.Epoch)
	})
}
func TestOwnerKernelForeignTable(t *testing.T) {
	isolated(t, func() {
		n := connectKernel(t)
		tab := n.CreateTable(kernelTable())
		ch := n.AddChain(&nftables.Chain{Name: "picketx_foreign", Table: tab})
		n.AddRule(&nftables.Rule{Table: tab, Chain: ch, UserData: []byte("foreign"), Exprs: []expr.Any{&expr.Counter{}}})
		if err := n.Flush(); err != nil {
			t.Fatal(err)
		}
		c := kernelConfig(t)
		s, err := Open(context.Background(), c)
		if s != nil {
			s.Close()
		}
		if err == nil {
			t.Fatal("foreign table adopted")
		}
		rules, err := n.GetRules(tab, ch)
		if err != nil || len(rules) != 1 || string(rules[0].UserData) != "foreign" {
			t.Fatalf("foreign table changed %v %v", rules, err)
		}
		if _, err = os.Stat(c.StatePath); !os.IsNotExist(err) {
			t.Fatalf("foreign claim published journal: %v", err)
		}
		t.Log("existing same-name foreign table and rule preserved")
	})
}
func TestOwnerKernelUnexpectedStructure(t *testing.T) {
	isolated(t, func() {
		for _, kind := range []string{"chain", "rule", "set", "object"} {
			t.Run(kind, func(t *testing.T) {
				c := kernelConfig(t)
				s := openKernel(t, c)
				s.Close()
				n := connectKernel(t)
				tab := kernelTable()
				switch kind {
				case "chain":
					n.AddChain(&nftables.Chain{Name: "picketx_extra", Table: tab})
				case "rule":
					n.AddRule(&nftables.Rule{Table: tab, Chain: &nftables.Chain{Name: ownerChain, Table: tab}, Exprs: []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}})
				case "set":
					if err := n.AddSet(&nftables.Set{Table: tab, Name: "picketx_extra", KeyType: nftables.TypeInteger}, nil); err != nil {
						t.Fatal(err)
					}
				case "object":
					n.AddObj(&nftables.CounterObj{Table: tab, Name: "picketx_extra"})
				}
				if err := n.Flush(); err != nil {
					t.Fatal(err)
				}
				s, err := Open(context.Background(), c)
				if s != nil {
					s.Close()
				}
				if err == nil {
					t.Fatal("unsupported structure adopted")
				}
				// Test-only teardown inside this case's disposable namespace.
				n.DelTable(tab)
				if err = n.Flush(); err != nil {
					t.Fatal(err)
				}
				t.Logf("real extra %s rejected without replacement", kind)
			})
		}
	})
}
func TestOwnerKernelExclusiveCreate(t *testing.T) {
	isolated(t, func() {
		n := connectKernel(t)
		reader, err := newStrictNetlink()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		b := &nftKernel{conn: n, reader: reader}
		live, err := b.Observe(context.Background())
		if err != nil || live.Exists {
			t.Fatalf("initial observation %+v %v", live, err)
		}
		competing := connectKernel(t)
		tab := competing.CreateTable(kernelTable())
		competing.AddChain(&nftables.Chain{Name: "picketx_competitor", Table: tab})
		if err = competing.Flush(); err != nil {
			t.Fatal(err)
		}
		r := scopeRecord(Config{ProviderID: "p", AgentRef: "a", RuntimeKey: "r"}, namespace{BootID: "b", Device: 1, Inode: 1})
		r.OwnerID, _ = randomID()
		r.Epoch, _ = randomID()
		if err = b.Create(context.Background(), markerFor(r)); err == nil {
			t.Fatal("exclusive create merged into foreign table")
		}
		chains, err := competing.ListChainsOfTableFamily(nftables.TableFamilyINet)
		if err != nil || len(chains) != 1 || chains[0].Name != "picketx_competitor" {
			t.Fatalf("exclusive rejection was not atomic: %+v %v", chains, err)
		}
		t.Logf("inspect/create race rejected atomically: %v", err)
	})
}

func TestOwnerKernelHelper(t *testing.T) {
	path := os.Getenv("PICKETX_OWNER_HELPER_PATH")
	if path == "" {
		return
	}
	if os.Getenv("PICKETX_OWNER_HELPER_MOUNT") == "1" {
		if err := unix.Mount("tmpfs", "/run", "tmpfs", 0, "mode=0755"); err != nil {
			t.Fatal(err)
		}
	}
	c := Config{StatePath: path, ProviderID: "static", AgentRef: "node", RuntimeKey: "host-scope"}
	s, err := Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var ready any = s.Status()
	if os.Getenv("PICKETX_OWNER_HELPER_ADMISSION") == "1" {
		ready = applyAdmission(t, s, dualAdmissionPolicy(t, true, false))
	}
	if os.Getenv("PICKETX_OWNER_HELPER_MARKS") == "1" {
		marks, e := s.ReconcileMarks(context.Background(), kernelMarkRange(), []markstate.Target{markTarget("a")})
		if e != nil {
			t.Fatal(e)
		}
		ready = markHelperReady{Owner: s.Status(), Marks: marks}
	}
	if err = json.NewEncoder(os.Stdout).Encode(ready); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	_, _ = os.Stdin.Read(one[:])
}
func TestOwnerKernelProcessLock(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		child := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestOwnerKernelHelper$", "-test.v")
		child.Env = append(os.Environ(), "PICKETX_OWNER_HELPER_PATH="+c.StatePath)
		stdout, err := child.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdin, err := child.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer stdin.Close()
		child.Stderr = os.Stderr
		if err = child.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if child.ProcessState == nil {
				child.Process.Kill()
				child.Wait()
			}
		}()
		reader := bufio.NewReader(stdout)
		var status Status
		for {
			line, e := reader.ReadBytes('\n')
			if e != nil {
				t.Fatalf("owner child exited before readiness: %s %v", line, e)
			}
			if json.Unmarshal(line, &status) == nil && status.OwnershipConfirmed {
				break
			}
		}
		other := c
		other.StatePath = filepath.Join(filepath.Dir(c.StatePath), "alternate.json")
		if s, e := Open(context.Background(), other); e == nil {
			s.Close()
			t.Fatal("second process path bypassed namespace singleton")
		}
		separateRun := exec.CommandContext(ctx, "unshare", "--mount", "--propagation", "private", os.Args[0], "-test.run", "^TestOwnerKernelHelper$", "-test.v")
		separateRun.Env = append(os.Environ(), "PICKETX_OWNER_HELPER_PATH="+other.StatePath, "PICKETX_OWNER_HELPER_MOUNT=1")
		out, mountErr := separateRun.CombinedOutput()
		if mountErr == nil || !strings.Contains(string(out), "namespace singleton socket") {
			t.Fatalf("separate /run lock proof failed: %s %v", out, mountErr)
		}
		if err = child.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		if err = child.Wait(); err == nil {
			t.Fatal("expected killed helper")
		}
		io.Copy(io.Discard, reader)
		s := openKernel(t, c)
		defer s.Close()
		if s.Status().OwnerID != status.OwnerID {
			t.Fatal("crash lost installation identity")
		}
		t.Log("real helper killed without Close; lock released and same marker recovered")
	})
}
func TestOwnerKernelPendingRecovery(t *testing.T) {
	isolated(t, func() {
		c := kernelConfig(t)
		pin, err := os.Open("/proc/thread-self/ns/net")
		if err != nil {
			t.Fatal(err)
		}
		ns, err := pinnedNamespace(pin)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := nftables.New(nftables.WithNetNSFd(int(pin.Fd())))
		if err != nil {
			t.Fatal(err)
		}
		boom := errors.New("crash before confirmed publication")
		reader, err := newStrictNetlink()
		if err != nil {
			t.Fatal(err)
		}
		_, err = openWith(context.Background(), c, environment{namespace: ns, lockRoot: runtimeLockRoot, backend: &nftKernel{conn: conn, reader: reader}, release: pin.Close, save: func(path string, r record) error {
			if r.Phase == "confirmed" {
				return boom
			}
			return writeRecord(path, r)
		}})
		if !errors.Is(err, boom) {
			t.Fatalf("unexpected failure %v", err)
		}
		pending, exists, err := readRecord(c.StatePath)
		if err != nil || !exists || pending.Phase != "pending" {
			t.Fatalf("pending recovery record %+v %v", pending, err)
		}
		s := openKernel(t, c)
		defer s.Close()
		if s.Status().Epoch != pending.Epoch {
			t.Fatal("pending marker was replaced")
		}
		t.Log(fmt.Sprintf("pending epoch %s confirmed from live kernel without re-create", pending.Epoch))
	})
}
