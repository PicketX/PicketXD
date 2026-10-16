// SPDX-License-Identifier: GPL-3.0-or-later
package hostpolicy

import "testing"

func TestCompleteInputOwnsReplacementHistory(t *testing.T) {
	in := input()
	p, e := CompileInput(in, scope(t))
	if e != nil {
		t.Fatal(e)
	}
	in.Networks[0].Port++
	copy, e := p.Input()
	if e != nil {
		t.Fatal(e)
	}
	if copy.Networks[0].Port == in.Networks[0].Port {
		t.Fatal("input alias")
	}
	copy.Networks[0].Port++
	again, e := p.Input()
	if e != nil || again.Networks[0].Port == copy.Networks[0].Port {
		t.Fatal("returned history alias", e)
	}
	if _, e = compiled(t, input()).Input(); e == nil {
		t.Fatal("plan projection falsely claimed complete history")
	}
}
