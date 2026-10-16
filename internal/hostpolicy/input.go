// SPDX-License-Identifier: GPL-3.0-or-later
package hostpolicy

import (
	"encoding/json"
	"fmt"

	"github.com/PicketX/PicketX/pkg/networkplan"
)

// CompileInput retains the complete desired input, including dormant and
// revoked contributions and unbound entries, for durable replacement checks.
// Compile(plan,scope) remains a projection-only API and cannot supply history.
func CompileInput(in networkplan.Input, scope *Scope) (*Policy, error) {
	data, e := json.Marshal(in)
	if e != nil {
		return nil, e
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("complete host input exceeds 8 MiB")
	}
	var owned networkplan.Input
	if e = json.Unmarshal(data, &owned); e != nil {
		return nil, e
	}
	plan, e := networkplan.Compile(owned)
	if e != nil {
		return nil, e
	}
	p, e := Compile(plan, scope)
	if e != nil {
		return nil, e
	}
	p.inputJSON = data
	return p, nil
}

// Input returns an independently owned complete input, never a live receipt.
func (p *Policy) Input() (networkplan.Input, error) {
	var in networkplan.Input
	if p == nil || !p.valid() || len(p.inputJSON) == 0 {
		return in, fmt.Errorf("complete input required; use hostpolicy.CompileInput for application")
	}
	e := json.Unmarshal(p.inputJSON, &in)
	return in, e
}
