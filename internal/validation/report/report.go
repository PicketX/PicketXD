// Package report preserves observed M0 evidence without promoting skipped work.
package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type ScenarioResult struct {
	Name, Status, Detail         string
	Selected, Deleted, Remaining int
}
type Report struct {
	Kernel, Go string
	Libraries  map[string]string
	Scenarios  []ScenarioResult
}

func Write(w io.Writer, r Report) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(r)
}
func CheckRequired(r Report, names []string) error {
	byName := make(map[string]string)
	for _, s := range r.Scenarios {
		if _, exists := byName[s.Name]; exists {
			return fmt.Errorf("duplicate scenario %s", s.Name)
		}
		byName[s.Name] = s.Status
	}
	var problems []error
	for _, n := range names {
		if byName[n] != "pass" {
			problems = append(problems, fmt.Errorf("required scenario %s: status %q", n, byName[n]))
		}
	}
	return errors.Join(problems...)
}
func ReadEvents(reader io.Reader) ([]ScenarioResult, error) {
	d := json.NewDecoder(reader)
	var out []ScenarioResult
	indices := make(map[string]int)
	for {
		var e struct{ Action, Test, Output string }
		err := d.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("test JSON: %w", err)
		}
		if e.Test == "" || strings.Contains(e.Test, "/") {
			continue
		}
		index, exists := indices[e.Test]
		if !exists {
			index = len(out)
			indices[e.Test] = index
			out = append(out, ScenarioResult{Name: e.Test, Status: "fail", Remaining: -1})
		}
		r := &out[index]
		switch e.Action {
		case "pass":
			r.Status = "pass"
		case "fail":
			r.Status = "fail"
		case "skip":
			r.Status = "skipped"
		case "output":
			if len(r.Detail) < 8192 {
				r.Detail += e.Output
			}
			for _, format := range []struct{ prefix, scan string }{
				{"revocation selected=", "revocation selected=%d deleted=%d remaining=%d"},
				{"result={Selected:", "result={Selected:%d Deleted:%d Remaining:%d}"},
			} {
				if pos := strings.Index(e.Output, format.prefix); pos >= 0 {
					var selected, deleted, remaining int
					if n, _ := fmt.Sscanf(e.Output[pos:], format.scan, &selected, &deleted, &remaining); n == 3 {
						r.Selected, r.Deleted, r.Remaining = selected, deleted, remaining
					}
				}
			}
		}
	}
	return out, nil
}

// RequiredScenarios is the acceptance gate, not a list of assumed successes.
var RequiredScenarios = []string{
	"TestDefaultDenyDualStack", "TestScopedAllowDualStack", "TestDenyPrecedence", "TestConcatenatedIntervalSets", "TestOverlapRetainsMembership", "TestIndependentDeadlines", "TestNativeTimeoutWithoutController", "TestGracefulExpiry", "TestRestartPreservesDeadline", "TestInvalidBatchPreservesPreviousState",
	"TestStrictRevokeDualStack", "TestStrictRevokePreservesOtherGrant", "TestStrictRevokePreservesUnrelatedFlow", "TestOldTCPPacketsCannotResurrect", "TestUDPAfterDeletionRequiresCurrentAllow", "TestNATScopedDeletion", "TestApplyFailureDoesNotTriggerDeletion",
}
