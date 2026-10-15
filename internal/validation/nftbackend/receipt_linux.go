package nftbackend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/PicketX/PicketXD/internal/validation/fixture"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

type Options struct{ FixtureNAT, HostInputFixture bool }

// ApplyError distinguishes a proven rejection from an uncertain transaction.
type ApplyError struct {
	Rejected bool
	Err      error
}

func (e *ApplyError) Error() string { return e.Err.Error() }
func (e *ApplyError) Unwrap() error { return e.Err }

type setExpectation struct {
	rule    fixture.Rule
	element nftables.SetElement
	set     *nftables.Set
	present bool
}
type pendingMember struct {
	rule fixture.Rule
	set  *nftables.Set
	deny bool
}
type manifest struct {
	chains  map[string]*nftables.Chain
	rules   map[string][][]expr.Any
	sets    map[string]setExpectation
	pending []pendingMember
}
type Receipt struct {
	Emitted, Omitted             fixture.Snapshot
	DispatchedAt, AcknowledgedAt time.Time
	owner                        *Backend
	generation                   uint64
	manifest                     *manifest
	proof                        *receiptProof
}
type receiptProof struct {
	emitted, omitted fixture.Snapshot
	dispatch, ack    time.Time
}

func cloneSnapshot(s fixture.Snapshot) fixture.Snapshot {
	return fixture.Snapshot{Protected: slices.Clone(s.Protected), Allow: slices.Clone(s.Allow), Deny: slices.Clone(s.Deny)}
}
func (b *Backend) ApplyReceipt(ctx context.Context, s fixture.Snapshot, opts Options) (Receipt, error) {
	if opts.FixtureNAT && opts.HostInputFixture {
		return Receipt{}, &ApplyError{true, fmt.Errorf("host INPUT and transit NAT fixtures cannot mix")}
	}
	if e := fixture.Validate(s); e != nil {
		return Receipt{}, &ApplyError{true, e}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return Receipt{}, &ApplyError{true, e}
	}
	return b.applyReceiptLocked(ctx, s, opts, b.now())
}
func (b *Backend) applyReceiptLocked(ctx context.Context, s fixture.Snapshot, opts Options, now time.Time) (Receipt, error) {
	// Each attempt owns a fresh message buffer. Failed preparation is discarded,
	// never retained in the shared query connection or replayed on a later Flush.
	c := &nftables.Conn{NetNS: b.conn.NetNS, TestDial: b.conn.TestDial}
	r := Receipt{Emitted: fixture.Snapshot{Protected: slices.Clone(s.Protected)}, DispatchedAt: now, owner: b, generation: b.generation + 1, manifest: &manifest{chains: make(map[string]*nftables.Chain), rules: make(map[string][][]expr.Any), sets: make(map[string]setExpectation)}}
	if e := b.queueBatch(ctx, c, s, now, &r, opts); e != nil {
		return Receipt{}, &ApplyError{true, e}
	}
	if e := ctx.Err(); e != nil {
		return Receipt{}, &ApplyError{true, e}
	}
	if b.inject != nil {
		b.inject(c)
	}
	// Structural preparation and list queries may block. Add membership last,
	// using fresh time inside the serialized dispatch path, not queue-entry time.
	r.DispatchedAt = b.now()
	for _, pending := range r.manifest.pending {
		element, active := membership(pending.rule, r.DispatchedAt)
		out := &r.Emitted.Allow
		missing := &r.Omitted.Allow
		if pending.deny {
			out = &r.Emitted.Deny
			missing = &r.Omitted.Deny
		}
		if !active {
			*missing = append(*missing, pending.rule)
			continue
		}
		if e := c.SetAddElements(pending.set, []nftables.SetElement{element}); e != nil {
			return Receipt{}, &ApplyError{true, e}
		}
		*out = append(*out, pending.rule)
		want := r.manifest.sets[pending.set.Name]
		want.element = element
		want.present = true
		r.manifest.sets[pending.set.Name] = want
	}
	if e := ctx.Err(); e != nil {
		return Receipt{}, &ApplyError{true, e}
	}
	if e := c.Flush(); e != nil {
		rejected := errors.Is(e, unix.EINVAL) || errors.Is(e, unix.EOPNOTSUPP) || errors.Is(e, unix.ENOENT) || errors.Is(e, unix.EEXIST) || errors.Is(e, unix.EPERM)
		return Receipt{}, &ApplyError{rejected, fmt.Errorf("apply owned batch: %w", e)}
	}
	r.AcknowledgedAt = b.now()
	r.proof = &receiptProof{cloneSnapshot(r.Emitted), cloneSnapshot(r.Omitted), r.DispatchedAt, r.AcknowledgedAt}
	b.generation++
	return r, nil
}
func sameExpressions(a, b []expr.Any) bool { return expressionDifference(a, b) == "" }
func expressionDifference(a, b []expr.Any) string {
	if len(a) != len(b) {
		return fmt.Sprintf("expression count %d/%d", len(a), len(b))
	}
	normalized := func(e expr.Any) expr.Any {
		// NFT_REG_1..4 are legacy 128-bit aliases of the same register bank
		// exposed by NFT_REG32_00..15. Kernel dumps prefer legacy numbers at
		// aligned boundaries; compare byte locations, preserving other fields.
		reg := func(v uint32) uint32 {
			if v >= 1 && v <= 4 {
				return 8 + (v-1)*4
			}
			return v
		}
		switch v := e.(type) {
		case *expr.Counter:
			return &expr.Counter{}
		case *expr.Lookup:
			c := *v
			c.SetID = 0
			c.SourceRegister = reg(c.SourceRegister)
			c.DestRegister = reg(c.DestRegister)
			return &c
		case *expr.Payload:
			c := *v
			c.DestRegister = reg(c.DestRegister)
			c.SourceRegister = reg(c.SourceRegister)
			return &c
		case *expr.Meta:
			c := *v
			c.Register = reg(c.Register)
			return &c
		case *expr.Immediate:
			c := *v
			c.Register = reg(c.Register)
			return &c
		case *expr.Bitwise:
			c := *v
			c.SourceRegister = reg(c.SourceRegister)
			c.DestRegister = reg(c.DestRegister)
			return &c
		case *expr.Cmp:
			c := *v
			c.Register = reg(c.Register)
			return &c
		case *expr.Range:
			c := *v
			c.Register = reg(c.Register)
			return &c
		case *expr.Ct:
			c := *v
			c.Register = reg(c.Register)
			return &c
		case *expr.NAT:
			c := *v
			c.RegAddrMin = reg(c.RegAddrMin)
			c.RegAddrMax = reg(c.RegAddrMax)
			c.RegProtoMin = reg(c.RegProtoMin)
			c.RegProtoMax = reg(c.RegProtoMax)
			return &c
		}
		return e
	}
	for i := range a {
		x, e := expr.Marshal(byte(nftables.TableFamilyINet), normalized(a[i]))
		y, f := expr.Marshal(byte(nftables.TableFamilyINet), normalized(b[i]))
		if e != nil || f != nil || !bytes.Equal(x, y) {
			return fmt.Sprintf("expression %d actual=%#v expected=%#v wire=%x/%x", i, a[i], b[i], x, y)
		}
	}
	return ""
}

// Confirm requires matching owned structure and membership. It tolerates only
// counters, kernel set IDs and legitimate native timeout disappearance.
func (b *Backend) Confirm(ctx context.Context, r Receipt) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.owner != b || r.generation != b.generation || r.manifest == nil {
		return fmt.Errorf("missing or stale acknowledged receipt")
	}
	if r.proof == nil || !reflect.DeepEqual(r.Emitted, r.proof.emitted) || !reflect.DeepEqual(r.Omitted, r.proof.omitted) || !r.DispatchedAt.Equal(r.proof.dispatch) || !r.AcknowledgedAt.Equal(r.proof.ack) {
		return fmt.Errorf("modified receipt cannot confirm application")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	tab := &nftables.Table{Name: "picketx", Family: nftables.TableFamilyINet}
	tables, e := b.conn.ListTablesOfFamily(tab.Family)
	if e != nil {
		return e
	}
	found := false
	for _, v := range tables {
		if v.Name == tab.Name {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("owned table missing")
	}
	chains, e := b.conn.ListChainsOfTableFamily(tab.Family)
	if e != nil {
		return e
	}
	count := 0
	for _, c := range chains {
		if c.Table.Name != tab.Name {
			continue
		}
		count++
		want, ok := r.manifest.chains[c.Name]
		if !ok {
			return fmt.Errorf("unexpected owned chain %q", c.Name)
		}
		equalPtr := func(a, b *nftables.ChainHook) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
		if !equalPtr(c.Hooknum, want.Hooknum) || c.Type != want.Type || ((c.Priority == nil) != (want.Priority == nil)) || c.Priority != nil && *c.Priority != *want.Priority {
			return fmt.Errorf("owned chain %s scope mismatch", c.Name)
		}
		rules, e := b.conn.GetRules(tab, c)
		if e != nil {
			return e
		}
		expected := r.manifest.rules[c.Name]
		if len(rules) != len(expected) {
			return fmt.Errorf("owned chain %s rule count differs", c.Name)
		}
		for i, v := range rules {
			if !sameExpressions(v.Exprs, expected[i]) {
				return fmt.Errorf("owned chain %s rule %d differs: %s", c.Name, i, expressionDifference(v.Exprs, expected[i]))
			}
		}
	}
	if count != len(r.manifest.chains) {
		return fmt.Errorf("missing owned chains")
	}
	sets, e := b.conn.GetSets(tab)
	if e != nil {
		return e
	}
	if len(sets) != len(r.manifest.sets) {
		return fmt.Errorf("owned set count differs: got %d want %d", len(sets), len(r.manifest.sets))
	}
	for _, set := range sets {
		want, ok := r.manifest.sets[set.Name]
		if !ok {
			return fmt.Errorf("unexpected owned permission set %s", set.Name)
		}
		if !set.Interval || !set.Concatenation || set.HasTimeout != !want.rule.ExpiresAt.IsZero() {
			return fmt.Errorf("set %s semantics differ", set.Name)
		}
		elements, e := b.conn.GetSetElements(set)
		if e != nil {
			return e
		}
		if e := confirmElements(set.Name, want, elements, b.now()); e != nil {
			return e
		}
	}
	return ctx.Err()
}

func confirmElements(name string, want setExpectation, elements []nftables.SetElement, now time.Time) error {
	if !want.present {
		if len(elements) != 0 {
			return fmt.Errorf("omitted set %s unexpectedly populated", name)
		}
		return nil
	}
	if len(elements) == 0 && !want.rule.ExpiresAt.IsZero() && !now.Add(time.Millisecond).Before(want.rule.ExpiresAt) {
		return nil
	}
	if len(elements) != 1 {
		return fmt.Errorf("set %s member count %d", name, len(elements))
	}
	got := elements[0]
	if !bytes.Equal(got.Key, want.element.Key) || !bytes.Equal(got.KeyEnd, want.element.KeyEnd) || got.IntervalEnd {
		return fmt.Errorf("set %s physical membership differs", name)
	}
	if !want.rule.ExpiresAt.IsZero() && (got.Timeout <= 0 || got.Expires <= 0 && want.rule.ExpiresAt.Sub(now) > time.Millisecond) {
		return fmt.Errorf("set %s lost bounded native timeout", name)
	}
	if !want.rule.ExpiresAt.IsZero() && (got.Expires > max(time.Duration(0), want.rule.ExpiresAt.Sub(now))+50*time.Millisecond || got.Timeout > want.element.Timeout) {
		return fmt.Errorf("set %s deadline widened", name)
	}
	return nil
}

// RemoveOwned handles explicit empty desired intent; it never invents a scope.
func (b *Backend) RemoveOwned(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return e
	}
	c := &nftables.Conn{NetNS: b.conn.NetNS, TestDial: b.conn.TestDial}
	tables, e := c.ListTablesOfFamily(nftables.TableFamilyINet)
	if e != nil {
		return e
	}
	found := false
	for _, tab := range tables {
		if tab.Name == "picketx" {
			c.DelTable(tab)
			found = true
		}
	}
	if found {
		if e := c.Flush(); e != nil {
			return e
		}
	}
	b.generation++
	return b.confirmAbsent(ctx)
}
func (b *Backend) ConfirmAbsent(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.confirmAbsent(ctx)
}
func (b *Backend) confirmAbsent(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	tables, e := b.conn.ListTablesOfFamily(nftables.TableFamilyINet)
	if e != nil {
		return e
	}
	for _, tab := range tables {
		if tab.Name == "picketx" {
			return fmt.Errorf("owned table still present")
		}
	}
	return ctx.Err()
}
