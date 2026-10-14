package nftbackend

import (
	"context"
	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"time"
)

const TestMark uint32 = 0x5049434b

type Backend struct {
	conn   *nftables.Conn
	inject func(*nftables.Conn)
	batch  func(context.Context, fixture.Snapshot, time.Time) error
}

func New(c *nftables.Conn) *Backend { return &Backend{conn: c} }
func (b *Backend) Apply(ctx context.Context, s fixture.Snapshot, now time.Time) error {
	if b.batch != nil {
		return b.batch(ctx, s, now)
	}
	return nil
}
func (b *Backend) Close() error { return nil }
func membership(fixture.Rule, time.Time) (nftables.SetElement, bool) {
	return nftables.SetElement{}, true
}
