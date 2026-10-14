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

func TestAcceptanceGateRejectsMissingSkippedAndFailedScenarios(t *testing.T) {
	for _, status := range []string{"skipped", "unsupported", "fail", ""} {
		r := Report{Scenarios: []ScenarioResult{{Name: "needed", Status: status}}}
		if err := CheckRequired(r, []string{"needed"}); err == nil {
			t.Fatalf("accepted %q", status)
		}
	}
	if err := CheckRequired(Report{}, []string{"needed"}); err == nil {
		t.Fatal("accepted unexecuted case")
	}
	if err := CheckRequired(Report{Scenarios: []ScenarioResult{{Name: "needed", Status: "pass"}}}, []string{"needed"}); err != nil {
		t.Fatal(err)
	}
}
func TestReadEventsPreservesFailureAndCounts(t *testing.T) {
	events := `{"Action":"run","Test":"strict"}
{"Action":"output","Test":"strict","Output":"revocation selected=2 deleted=1 remaining=1\n"}
{"Action":"fail","Test":"strict"}
{"Action":"run","Test":"not opted in"}
{"Action":"skip","Test":"not opted in"}
{"Action":"run","Test":"interrupted"}
`
	got, err := ReadEvents(bytes.NewBufferString(events))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Status != "fail" || got[0].Selected != 2 || got[0].Deleted != 1 || got[0].Remaining != 1 || got[1].Status != "skipped" || got[2].Status != "fail" {
		t.Fatalf("lost observed evidence: %+v", got)
	}
}
