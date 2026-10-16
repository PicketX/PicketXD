// SPDX-License-Identifier: GPL-3.0-or-later
package hostowner

import (
	"context"
	"fmt"
)

type RuntimeScope struct{ BootID, NetworkSpaceID string }

// RuntimeScope derives configuration facts from the held namespace pin, never
// a later /proc/self path lookup which could describe a different OS thread.
func (s *Session) RuntimeScope(ctx context.Context) (RuntimeScope, error) {
	if s == nil {
		return RuntimeScope{}, fmt.Errorf("ownership session required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(ctx); e != nil {
		return RuntimeScope{}, e
	}
	return RuntimeScope{BootID: s.record.BootID, NetworkSpaceID: fmt.Sprintf("%s/net:%d:%d", s.record.BootID, s.record.Device, s.record.Inode)}, nil
}
