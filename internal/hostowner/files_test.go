// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"testing"
)

func TestOwnershipUncertainDirectorySync(t *testing.T) {
	c, e, _ := setup(t)
	s := mustOpen(t, c, e)
	s.Close()
	r, _, err := readRecord(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	r.Phase = "pending"
	err = writeRecordWithSync(c.StatePath, r, func(string) error { return errors.New("fsync failed") })
	var published *publicationError
	if !errors.As(err, &published) || !published.Published {
		t.Fatalf("post-rename outcome lost: %v", err)
	}
	got, exists, err := readRecord(c.StatePath)
	if err != nil || !exists || got.Phase != "pending" {
		t.Fatalf("published state %+v %v %v", got, exists, err)
	}
	if info, err := os.Stat(c.StatePath); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private replacement %v %v", info, err)
	}
}

func makeFIFO(t *testing.T, path string) {
	t.Helper()
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
}
