// SPDX-License-Identifier: GPL-3.0-or-later
package hostrevoke

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCompactHistoryDeduplicatesAndOwnsRecordedFlows(t *testing.T) {
	raw := rawPolicy(t)
	old, marks := testImage(t, raw, nil)
	h, e := Remember(nil, old)
	if e != nil {
		t.Fatal(e)
	}
	if e = h.Validate(marks); e != nil {
		t.Fatal(e)
	}
	original, _ := json.Marshal(h)
	for i := 0; i < 40; i++ {
		renew := strings.ReplaceAll(raw, "grant-loopback", "grant-renew"+time.Unix(int64(i+1), 0).Format("150405"))
		im, _ := testImage(t, replaceValidity(renew, time.Unix(2000000000, 0), time.Unix(2000000100+int64(i), 0)), &marks)
		next, e := Remember(&h, im)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(h, next) {
			t.Fatal("renewal UID/deadline grew ownership history")
		}
	}
	current, marks := testImage(t, noGrant(raw), &marks)
	p, e := NewRecorded(h, current, marks)
	if e != nil {
		t.Fatal(e)
	}
	var flows []Flow
	for _, v6 := range []bool{false, true} {
		for _, proto := range []uint8{6, 17} {
			flows = append(flows, testFlow(old, v6, proto))
		}
	}
	h.Targets[0].Members = nil
	got, e := p.Select(flows, time.Now())
	if e != nil || len(got) != 4 {
		t.Fatal("recorded plan aliases history or lost original scope", got, e)
	}
	if e = json.Unmarshal(original, &h); e != nil {
		t.Fatal(e)
	}
	h2, e := Remember(&h, current)
	if e != nil || !reflect.DeepEqual(h, h2) {
		t.Fatal("revocation forgot historical membership", e)
	}
	old.Targets[0].Allow = nil
	if b, _ := json.Marshal(h); string(b) != string(original) {
		t.Fatal("caller image mutation changed history")
	}
}
func TestCompactHistoryRejectsCorruptionAndCrossScope(t *testing.T) {
	im, m := testImage(t, rawPolicy(t), nil)
	h, e := Remember(nil, im)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(h)
	for _, kind := range []string{"mark", "shape", "endpoint", "source family", "duplicate member", "missing protected", "scope", "duplicate target", "version"} {
		t.Run(kind, func(t *testing.T) {
			var x History
			json.Unmarshal(raw, &x)
			switch kind {
			case "mark":
				x.Targets[0].Mark = 999
			case "shape":
				x.Targets[0].Identity.Shape = strings.Repeat("a", 64)
			case "endpoint":
				x.Targets[0].Members[0].Endpoint.Port++
			case "source family":
				if x.Targets[0].Members[0].Source.Addr().Is4() {
					x.Targets[0].Members[0].Source = netip.MustParsePrefix("::/0")
				} else {
					x.Targets[0].Members[0].Source = netip.MustParsePrefix("0.0.0.0/0")
				}
			case "duplicate member":
				x.Targets[0].Members = append(x.Targets[0].Members, x.Targets[0].Members[0])
			case "missing protected":
				x.Targets[0].Protected = nil
			case "scope":
				x.Context.ID = "foreign"
			case "duplicate target":
				x.Targets = append(x.Targets, x.Targets[0])
			case "version":
				x.Version = 2
			}
			if e := x.Validate(m); e == nil {
				t.Fatal("corrupt compact history accepted")
			}
		})
	}
	other := im
	other.Context.ID = "foreign"
	if _, e = Remember(&h, other); e == nil {
		t.Fatal("cross context remembered")
	}
	if _, e = NewRecorded(h, other, m); e == nil {
		t.Fatal("cross scope plan")
	}
	x := h
	x.Targets = make([]HistoricalTarget, MaxHistoryTargets+1)
	if e = x.Validate(m); e == nil {
		t.Fatal("unbounded targets")
	}
	x = h
	x.Targets = append([]HistoricalTarget{}, h.Targets...)
	x.Targets[0].Members = make([]HistoricalMember, MaxHistoryMembers+1)
	if e = x.Validate(m); e == nil {
		t.Fatal("unbounded members")
	}
}
