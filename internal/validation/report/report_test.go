package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBridgeGateRequiresEveryNewKernelCase(t *testing.T) {
	var scenarios []ScenarioResult
	for _, name := range RequiredScenarios {
		scenarios = append(scenarios, ScenarioResult{Name: name, Status: "pass"})
	}
	if err := CheckRequired(Report{Scenarios: scenarios}, RequiredBridgeScenarios); err == nil {
		t.Fatal("old M0 passes satisfied new bridge gate")
	}
	for _, name := range RequiredBridgeScenarios {
		scenarios = append(scenarios, ScenarioResult{Name: name, Status: "pass"})
	}
	if err := CheckRequired(Report{Scenarios: scenarios}, RequiredBridgeScenarios); err != nil {
		t.Fatal(err)
	}
	scenarios[len(scenarios)-1].Status = "skipped"
	if err := CheckRequired(Report{Scenarios: scenarios}, RequiredBridgeScenarios); err == nil {
		t.Fatal("skipped bridge accepted")
	}
}

func TestSubtestEvidenceSurvivesAndSkippedVariantFailsGate(t *testing.T) {
	for _, status := range []string{"pass", "skip"} {
		events := fmt.Sprintf("{\"Action\":\"run\",\"Test\":\"TestBridgeEmptyOwnership\"}\n{\"Action\":\"run\",\"Test\":\"TestBridgeEmptyOwnership/strict\"}\n{\"Action\":\"output\",\"Test\":\"TestBridgeEmptyOwnership/strict\",\"Output\":\"epoch=live revocation selected=2 deleted=2 remaining=0\"}\n{\"Action\":%q,\"Test\":\"TestBridgeEmptyOwnership/strict\"}\n{\"Action\":\"pass\",\"Test\":\"TestBridgeEmptyOwnership\"}\n", status)
		got, e := ReadEvents(strings.NewReader(events))
		if e != nil || len(got) != 1 || !strings.Contains(got[0].Detail, "epoch=live") {
			t.Fatal(got, e)
		}
		e = CheckRequired(Report{Scenarios: got}, []string{"TestBridgeEmptyOwnership"})
		if (e == nil) != (status == "pass") {
			t.Fatal("skipped variant accepted", got, e)
		}
	}
}
func TestDuplicateTopLevelRunRejected(t *testing.T) {
	events := "{\"Action\":\"run\",\"Test\":\"required\"}\n{\"Action\":\"pass\",\"Test\":\"required\"}\n{\"Action\":\"run\",\"Test\":\"required\"}\n{\"Action\":\"pass\",\"Test\":\"required\"}\n"
	if _, e := ReadEvents(strings.NewReader(events)); e == nil {
		t.Fatal("duplicate observed test hidden")
	}
}

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

// These are the actual failure records emitted by the integration assertions.
func TestReadEventsPreservesActualPartialFailureRecord(t *testing.T) {
	for _, prefix := range []string{"strict", "NAT"} {
		events := fmt.Sprintf(`{"Action":"run","Test":"partial"}
{"Action":"output","Test":"partial","Output":"%s result={Selected:2 Deleted:1 Remaining:1} err=delete denied"}
{"Action":"fail","Test":"partial"}
`, prefix)
		got, err := ReadEvents(strings.NewReader(events))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Status != "fail" || got[0].Selected != 2 || got[0].Deleted != 1 || got[0].Remaining != 1 {
			t.Fatalf("partial failure counts lost: %+v", got)
		}
	}
}
