// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
	"bytes"
	"strings"
	"testing"
)

func TestEnforceHelpExposesExplicitMutationScope(t *testing.T) {
	var out bytes.Buffer
	c := New(&out, nil)
	c.SetArgs([]string{"enforce", "--help"})
	if e := c.Execute(); e != nil {
		t.Fatal(e)
	}
	for _, flag := range []string{"--state-dir", "--mark-first", "--mark-last", "--destination", "--once", "--handling"} {
		if !strings.Contains(out.String(), flag) {
			t.Fatal("missing enforcement setting", flag, out.String())
		}
	}
}
func TestEnforceRejectsFlagsBeforeNamespaceEffects(t *testing.T) {
	base := []string{"enforce", "--policy", "policy.yaml", "--state-dir", "relative", "--provider", "static", "--agent", "node", "--destination", "127.0.0.1", "--mark-first", "0x50490000", "--mark-last", "0x50490007"}
	for _, args := range [][]string{{"enforce"}, base, append(append([]string{}, base...), "--poll", "0s"), {"enforce", "--mark-first", "-1"}, {"enforce", "--strict"}} {
		var out bytes.Buffer
		c := New(&out, nil)
		c.SetArgs(args)
		if e := c.Execute(); e == nil || out.Len() != 0 {
			t.Fatal(args, out.String(), e)
		}
	}
}

func TestEnforceRejectsUnknownHandling(t *testing.T) {
	var out bytes.Buffer
	c := New(&out, nil)
	c.SetArgs([]string{"enforce", "--policy", "policy.yaml", "--state-dir", t.TempDir(), "--provider", "static", "--agent", "node", "--destination", "127.0.0.1", "--mark-first", "100", "--mark-last", "107", "--handling", "invalid"})
	if e := c.Execute(); e == nil || !strings.Contains(e.Error(), "handling") || out.Len() != 0 {
		t.Fatal(e, out.String())
	}
}
