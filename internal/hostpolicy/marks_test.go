// SPDX-License-Identifier: GPL-3.0-or-later
package hostpolicy

import (
	"github.com/PicketX/PicketXD/internal/markstate"
	"reflect"
	"testing"
)

func markTargets(t *testing.T, p *Policy) []markstate.Target {
	t.Helper()
	targets, e := p.MarkTargets()
	if e != nil || len(targets) != 2 {
		t.Fatalf("missing complete mark targets %v %v", targets, e)
	}
	for _, target := range targets {
		if e := target.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	return targets
}
func TestPolicyMarkIdentity(t *testing.T) {
	in := input()
	original := markTargets(t, compiled(t, in))
	in.Logical.Grants = nil
	in.Bindings[0].ID = "another-binding"
	if got := markTargets(t, compiled(t, in)); !reflect.DeepEqual(got, original) {
		t.Fatal("membership/Binding rotated marks", got, original)
	}
	in = input()
	in.Networks[0].Port = 9443
	changed := markTargets(t, compiled(t, in))
	for i, a := range changed {
		if a.Key.Entry.EndpointName == "web" {
			if a.Shape == original[i].Shape {
				t.Fatal("physical revision reused shape")
			}
		} else if a != original[i] {
			t.Fatal("unrelated endpoint rotated")
		}
	}
	in = input()
	in.Logical.Resources[0].Name = "new-uid"
	for i := range in.Networks {
		in.Networks[i].Entry.ResourceName = "new-uid"
	}
	in.Bindings[0].Resources[0].ResourceName = "new-uid"
	for i := range in.Logical.Grants {
		in.Logical.Grants[i].Scopes[0].ResourceName = "new-uid"
	}
	if got := markTargets(t, compiled(t, in)); got[0].Key == original[0].Key {
		t.Fatal("resource UID lost")
	}
	p := compiled(t, input())
	got := markTargets(t, p)
	got[0].Shape = "changed"
	if reflect.DeepEqual(got, markTargets(t, p)) {
		t.Fatal("request mutation changed policy")
	}
	if _, e := (*Policy)(nil).MarkTargets(); e == nil {
		t.Fatal("nil policy accepted")
	}
	in = input()
	in.Bindings = nil
	empty, e := compiled(t, in).MarkTargets()
	if e != nil || len(empty) != 0 {
		t.Fatal("empty protection allocated marks", empty, e)
	}
}
