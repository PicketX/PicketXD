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
	started := make(map[string]bool)
	childFailures := make(map[string]string)
	for {
		var e struct{ Action, Test, Output string }
		err := d.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("test JSON: %w", err)
		}
		if e.Test == "" {
			continue
		}
		if e.Action == "run" {
			if started[e.Test] {
				return nil, fmt.Errorf("duplicate test execution %s", e.Test)
			}
			started[e.Test] = true
		}
		name := strings.SplitN(e.Test, "/", 2)[0]
		child := name != e.Test
		index, exists := indices[name]
		if !exists {
			index = len(out)
			indices[name] = index
			out = append(out, ScenarioResult{Name: name, Status: "fail", Remaining: -1})
		}
		r := &out[index]
		if child && e.Action != "output" {
			if e.Action == "fail" {
				childFailures[name] = "fail"
			} else if e.Action == "skip" && childFailures[name] != "fail" {
				childFailures[name] = "skipped"
			}
			continue
		}
		switch e.Action {
		case "pass":
			r.Status = "pass"
			if status := childFailures[name]; status != "" {
				r.Status = status
			}
		case "fail":
			r.Status = "fail"
		case "skip":
			r.Status = "skipped"
		case "output":
			if len(r.Detail) < 65536 {
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

// RequiredBridgeScenarios must execute against generated shared intent in real
// disposable namespaces. Old M0 evidence cannot satisfy this additional gate.
var RequiredBridgeScenarios = []string{
	"TestBridgeGiteaIndependentDeadlines", "TestBridgeDenyAndRevocation", "TestBridgeUDP", "TestBridgeNativeExpiry", "TestBridgeFutureBoundaries", "TestBridgeReplayLateApply", "TestBridgeHistoricalStrictCleanup", "TestBridgeStrictPreservation", "TestBridgeApplyFailure", "TestBridgeConfirmationFailure", "TestBridgePartialCleanup", "TestBridgeBindingOwnership", "TestBridgeEmptyOwnership", "TestBridgeNAT", "TestBridgeInterruptedDump", "TestBridgeBackendDispatchDeadline", "TestBridgeBackendSubmillisecondReceipt", "TestBridgeNATAtomicRejection", "TestBridgeRejectsPermanentNativeMutation",
}

// RequiredHostScenarios execute accepted static publications on the real INPUT
// path. Transit bridge evidence cannot substitute for these host observations.
var RequiredHostScenarios = []string{
	"TestHostConfigurationDefaultDeny", "TestHostConfigurationSourcesAndDeny", "TestHostConfigurationExpiryWithInvalidReload", "TestHostConfigurationRecoveryDeadline", "TestHostOriginalDestinationDNAT", "TestHostUntrackedProtectedTraffic", "TestHostInputDoesNotClaimTransit", "TestHostPolicyCompilerPorts",
}

// RequiredOwnerScenarios prove ownership, mark history and the production
// Session admission engine. They supplement all prior packet/conformance gates.
var RequiredOwnerScenarios = []string{
	"TestOwnerKernelSocketExclusion",
	"TestOwnerKernelFreshAndRestart", "TestOwnerKernelForeignTable", "TestOwnerKernelUnexpectedStructure", "TestOwnerKernelExclusiveCreate", "TestOwnerKernelProcessLock", "TestOwnerKernelPendingRecovery",
	"TestOwnerKernelAdmissionUDPFutureAllow", "TestOwnerKernelAdmissionOriginalDNAT", "TestOwnerKernelAdmissionDualStack", "TestOwnerKernelAdmissionAbsoluteTime", "TestOwnerKernelAdmissionFutureDeny", "TestOwnerKernelAdmissionRecoveryAndDrift", "TestOwnerKernelAdmissionAtomicRejection", "TestOwnerKernelAdmissionWithdrawal", "TestOwnerKernelAdmissionProcessRecovery",
	"TestOwnerKernelMarkLifecycle", "TestOwnerKernelMarkCollision", "TestOwnerKernelMarkProcessRecovery", "TestOwnerKernelMarkDrift",
	"TestOwnerKernelCleanupPreservation", "TestOwnerKernelCleanupNATZoneID", "TestOwnerKernelCleanupPreflightDrift", "TestOwnerKernelCleanupPartialFailure",
	"TestOwnerKernelHistoryRestartCleanup", "TestOwnerKernelHistoryPendingRecovery", "TestOwnerKernelHistoryLegacyRefusal", "TestOwnerKernelHistoryCorruptionRefusal", "TestOwnerKernelHistoryProcessRecovery",
	"TestOwnerKernelStrictMembership", "TestOwnerKernelStrictPhysicalReplacement", "TestOwnerKernelStrictPendingRecovery", "TestOwnerKernelStrictReverseReconstruction", "TestOwnerKernelStrictOriginalDNAT",
}

// RequiredAgentScenarios execute the actual prebuilt standalone command.
var RequiredAgentScenarios = []string{"TestEnforceProcessReloadDualStack", "TestEnforceProcessExpiryAndRecovery", "TestEnforceProcessFutureDeny", "TestEnforceProcessFutureAllow", "TestEnforceProcessStartupRefusals", "TestEnforceProcessDrift", "TestEnforceProcessForeignAndInvalidOnce", "TestEnforceProcessStrictRevocation", "TestEnforceProcessStrictExpiryWithoutProcess", "TestEnforceProcessStrictRegrantAfterOfflineExpiry", "TestEnforceProcessControllerApplyWithdraw", "TestEnforceProcessControllerStrictOfflineRecovery"}
