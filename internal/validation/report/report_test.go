package report

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestReportPreservesIncompleteAndSkippedResults(t *testing.T) {
	r := Report{Kernel: "test-kernel", Go: "test-go", Libraries: map[string]string{"netlink": "v1.3.1"}, Scenarios: []ScenarioResult{{Name: "partial", Status: "fail", Selected: 2, Deleted: 1, Remaining: 1}, {Name: "not opted in", Status: "skipped"}, {Name: "kernel feature", Status: "unsupported"}, {Name: "observed", Status: "pass"}}}
	var out bytes.Buffer
	if err := Write(&out, r); err != nil {
		t.Fatal(err)
	}
	var got Report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Scenarios) != 4 || got.Scenarios[0].Status != "fail" || got.Scenarios[0].Remaining != 1 || got.Scenarios[1].Status != "skipped" || got.Scenarios[2].Status != "unsupported" || got.Scenarios[3].Status != "pass" {
		t.Fatalf("statuses/counts changed: %+v", got)
	}
}
