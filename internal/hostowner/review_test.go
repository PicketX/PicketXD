// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"encoding/binary"
	nl "github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

type dumpFunc func(context.Context, uint16, []nl.Attribute) ([]nl.Message, error)

func (f dumpFunc) Dump(ctx context.Context, cmd uint16, a []nl.Attribute) ([]nl.Message, error) {
	return f(ctx, cmd, a)
}
func (dumpFunc) Close() error { return nil }

func TestRawDumpInterruptedCompletion(t *testing.T) {
	req := nl.Message{Header: nl.Header{Sequence: 7, PID: 9}}
	for _, typ := range []nl.HeaderType{nl.Done, nl.HeaderType((unix.NFNL_SUBSYS_NFTABLES << 8) | unix.NFT_MSG_NEWTABLE)} {
		message := nl.Message{Header: nl.Header{Type: typ, Flags: nl.Multi | nl.DumpInterrupted, Sequence: 7, PID: 9, Length: 20}, Data: []byte{0, 0, 0, 0}}
		data, err := message.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = decodeDumpDatagram(data, req); err == nil {
			t.Fatal("wire DUMP_INTR accepted as complete")
		}
	}
}
func TestRawMarkerRejectsUnsupportedExpression(t *testing.T) {
	counter, _ := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_EXPR_NAME, Data: []byte("counter\x00")}, {Type: unix.NFTA_EXPR_DATA, Data: []byte{}}})
	unknown, _ := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_EXPR_NAME, Data: []byte("socket\x00")}, {Type: unix.NFTA_EXPR_DATA, Data: []byte{}}})
	for _, extra := range []bool{false, true} {
		elems := []nl.Attribute{{Type: unix.NFTA_LIST_ELEM, Data: counter}}
		if extra {
			elems = append(elems, nl.Attribute{Type: unix.NFTA_LIST_ELEM, Data: unknown})
		}
		raw, _ := nl.MarshalAttributes(elems)
		err := checkRawCounter(raw)
		if !extra && err != nil {
			t.Fatal(err)
		}
		if extra && err == nil {
			t.Fatal("unknown wire expression disappeared from marker rule")
		}
	}
}
func TestDirectoryCreationSymlinkRace(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "predictable")
	stateDir := filepath.Join(link, "private")
	hook := func(fd int, name string, mode uint32) error {
		if name == "predictable" || name == stateDir {
			if err := os.Symlink(target, link); err != nil {
				return err
			}
		}
		return unix.Mkdirat(fd, name, mode)
	}
	f, err := walkDirectory(stateDir, hook)
	if f != nil {
		f.Close()
	}
	if err == nil {
		t.Fatal("creation race followed a symlink")
	}
	if _, err = os.Stat(filepath.Join(target, "private")); !os.IsNotExist(err) {
		t.Fatal("created a directory through a raced symlink", err)
	}
}
func TestJournalUsesPinnedDirectory(t *testing.T) {
	c, e, _ := setup(t)
	s := mustOpen(t, c, e)
	s.Close()
	r, _, err := readRecord(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(c.StatePath)
	fd, err := walkDirectory(dir, unix.Mkdirat)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	outside := filepath.Join(t.TempDir(), "outside")
	os.Mkdir(outside, 0700)
	os.Rename(dir, dir+".old")
	os.Symlink(outside, dir)
	j := journal{dir: fd, name: filepath.Base(c.StatePath), path: c.StatePath}
	r.Phase = "pending"
	if err = j.write(r, nil); err != nil {
		t.Fatal("pinned publication followed renamed path", err)
	}
	if _, err = os.Stat(filepath.Join(outside, j.name)); !os.IsNotExist(err) {
		t.Fatal("journal redirected outside pinned directory")
	}
}

// Verify malformed completion/error payloads do not become a successful empty dump.
func TestRawDumpMalformedCompletion(t *testing.T) {
	req := nl.Message{Header: nl.Header{Sequence: 7, PID: 9}}
	for _, data := range [][]byte{{1}, binary.NativeEndian.AppendUint32(nil, uint32(unix.EPERM))} {
		msg := nl.Message{Header: nl.Header{Type: nl.Done, Flags: nl.Multi, Sequence: 7, PID: 9, Length: uint32(16 + len(data))}, Data: data}
		// Encode an aligned datagram with the actual (possibly unaligned)
		// header length, as a malformed kernel reply would appear on the wire.
		msg.Header.Length = 20
		wire, marshalErr := msg.MarshalBinary()
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		binary.NativeEndian.PutUint32(wire[:4], uint32(16+len(data)))
		if _, _, err := decodeDumpDatagram(wire, req); err == nil {
			t.Fatal("bad DONE accepted")
		}
	}
}

func TestStrictWireObservationAndFailures(t *testing.T) {
	r := scopeRecord(Config{ProviderID: "p", AgentRef: "a", RuntimeKey: "r"}, namespace{BootID: "b", Device: 1, Inode: 2})
	r.OwnerID = "0123456789abcdef0123456789abcdef"
	r.Epoch = r.OwnerID
	counter, _ := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_EXPR_NAME, Data: []byte("counter\x00")}, {Type: unix.NFTA_EXPR_DATA, Data: nil}})
	unknown, _ := nl.MarshalAttributes([]nl.Attribute{{Type: unix.NFTA_EXPR_NAME, Data: []byte("socket\x00")}, {Type: unix.NFTA_EXPR_DATA, Data: nil}})
	for _, which := range []string{"valid", "unknown-expression", "interrupted-empty-set", "duplicate-marker", "extra-object"} {
		t.Run(which, func(t *testing.T) {
			reader := dumpFunc(func(_ context.Context, cmd uint16, _ []nl.Attribute) ([]nl.Message, error) {
				var a []nl.Attribute
				switch cmd {
				case unix.NFT_MSG_GETTABLE:
					a = []nl.Attribute{{Type: unix.NFTA_TABLE_NAME, Data: []byte("picketx\x00")}, {Type: unix.NFTA_TABLE_FLAGS, Data: []byte{0, 0, 0, 0}}, {Type: unix.NFTA_TABLE_USE, Data: []byte{0, 0, 0, 1}}}
				case unix.NFT_MSG_GETCHAIN:
					a = []nl.Attribute{{Type: unix.NFTA_CHAIN_TABLE, Data: []byte("picketx\x00")}, {Type: unix.NFTA_CHAIN_NAME, Data: []byte(ownerChain + "\x00")}}
				case unix.NFT_MSG_GETRULE:
					elems := []nl.Attribute{{Type: unix.NFTA_LIST_ELEM, Data: counter}}
					if which == "unknown-expression" {
						elems = append(elems, nl.Attribute{Type: unix.NFTA_LIST_ELEM, Data: unknown})
					}
					expressions, _ := nl.MarshalAttributes(elems)
					a = []nl.Attribute{{Type: unix.NFTA_RULE_TABLE, Data: []byte("picketx\x00")}, {Type: unix.NFTA_RULE_CHAIN, Data: []byte(ownerChain + "\x00")}, {Type: unix.NFTA_RULE_EXPRESSIONS, Data: expressions}, {Type: unix.NFTA_RULE_USERDATA, Data: []byte(markerFor(r))}}
				case unix.NFT_MSG_GETSET:
					if which == "interrupted-empty-set" {
						req := nl.Message{Header: nl.Header{Sequence: 1, PID: 2}}
						msg := nl.Message{Header: nl.Header{Type: nl.Done, Flags: nl.Multi | nl.DumpInterrupted, Sequence: 1, PID: 2, Length: 20}, Data: make([]byte, 4)}
						wire, _ := msg.MarshalBinary()
						_, _, err := decodeDumpDatagram(wire, req)
						return nil, err
					}
					return nil, nil
				case unix.NFT_MSG_GETOBJ:
					if which == "extra-object" {
						a = []nl.Attribute{{Type: 1, Data: []byte("picketx\x00")}}
					} else {
						return nil, nil
					}
				default:
					return nil, nil
				}
				data, _ := nl.MarshalAttributes(a)
				m := nl.Message{Data: append([]byte{unix.NFPROTO_INET, 0, 0, 0}, data...)}
				messages := []nl.Message{m}
				if which == "duplicate-marker" && cmd == unix.NFT_MSG_GETRULE {
					messages = append(messages, m)
				}
				return messages, nil
			})
			got, err := strictObserve(context.Background(), reader)
			if which == "valid" {
				if err != nil || got.Marker != markerFor(r) {
					t.Fatalf("valid strict wire rejected: %+v %v", got, err)
				}
			} else if err == nil {
				t.Fatal("invalid wire state accepted")
			}
		})
	}
}

func TestInvalidOwnershipIdentityRejectedBeforeFiles(t *testing.T) {
	for _, which := range []string{"root-state-path", "invalid-utf8"} {
		t.Run(which, func(t *testing.T) {
			c, e, b := setup(t)
			if which == "root-state-path" {
				c.StatePath = "/"
			} else {
				c.ProviderID = string([]byte{0xff})
			}
			s, err := openWith(context.Background(), c, e)
			if s != nil {
				s.Close()
			}
			if err == nil || b.creates != 0 {
				t.Fatal("invalid identity had effects")
			}
		})
	}
}
