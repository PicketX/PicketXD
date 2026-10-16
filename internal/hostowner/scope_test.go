// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeScopeUsesHeldIdentity(t *testing.T) {
	c, e, _ := setup(t)
	s := mustOpen(t, c, e)
	first, err := s.RuntimeScope(context.Background())
	if err != nil || first.BootID != "boot-1" || first.NetworkSpaceID == "" {
		t.Fatal(first, err)
	}
	again, err := s.RuntimeScope(context.Background())
	if err != nil || again != first {
		t.Fatal(again, err)
	}
	boom := errors.New("namespace pin changed")
	s.revalidate = func() error { return boom }
	if _, err = s.RuntimeScope(context.Background()); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	s.revalidate = nil
	s.faulted = true
	if _, err = s.RuntimeScope(context.Background()); err == nil {
		t.Fatal("faulted scope accepted")
	}
	s.faulted = false
	s.Close()
	if _, err = s.RuntimeScope(context.Background()); err == nil {
		t.Fatal("closed scope accepted")
	}
}
