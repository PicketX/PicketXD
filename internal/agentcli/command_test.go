// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareOnceRecoversOnlyItsOwnState(t *testing.T) {
	raw, e := os.ReadFile("../../examples/prepare/policy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	policy := filepath.Join(dir, "policy.yaml")
	state := filepath.Join(dir, "private", "cache.json")
	os.WriteFile(policy, raw, 0600)
	args := []string{"prepare", "--once", "--policy", policy, "--state", state, "--provider", "static-example", "--agent", "agent-a", "--destination", "192.0.2.100", "--destination", "2001:db8::100"}
	run := func(a []string) (string, error) {
		var out bytes.Buffer
		c := New(&out, slog.New(slog.NewTextHandler(io.Discard, nil)))
		c.SetArgs(a)
		e := c.ExecuteContext(context.Background())
		return out.String(), e
	}
	out, e := run(args)
	if e != nil {
		t.Fatal(e)
	}
	var report struct {
		State                                    string `json:"state"`
		Prepared, EnforcementConfirmed, Restored bool
		Generation                               uint64
		LastError                                string
	}
	if e = json.Unmarshal([]byte(out), &report); e != nil {
		t.Fatal(e)
	}
	if report.State != "prepared" || !report.Prepared || report.EnforcementConfirmed || report.Restored || report.Generation != 1 {
		t.Fatal(out)
	}
	out, e = run(args)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal([]byte(out), &report)
	if !report.Restored || report.Generation != 1 {
		t.Fatal("restart changed accepted revision", out)
	}
	os.WriteFile(policy, []byte("broken"), 0600)
	out, e = run(args)
	if e == nil {
		t.Fatal("invalid current publication exited successfully")
	}
	json.Unmarshal([]byte(out), &report)
	if !report.Restored || report.LastError == "" || report.EnforcementConfirmed {
		t.Fatal("fallback hid failure", out)
	}
	foreign := append([]string(nil), args...)
	for i := range foreign {
		if foreign[i] == "static-example" {
			foreign[i] = "other"
		}
	}
	if out, e = run(foreign); e == nil || out != "" {
		t.Fatal("foreign cache became authoritative")
	}
}
func TestPrepareRejectsIncompleteOrUnsupportedFlags(t *testing.T) {
	for _, args := range [][]string{{"prepare"}, {"prepare", "--poll", "0s"}, {"prepare", "--mode", "controller"}} {
		var out bytes.Buffer
		c := New(&out, slog.New(slog.NewTextHandler(io.Discard, nil)))
		c.SetArgs(args)
		if e := c.Execute(); e == nil || out.Len() != 0 {
			t.Fatalf("flags accepted: %#v", args)
		}
	}
}
