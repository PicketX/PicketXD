// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"context"
	"errors"
	"fmt"
	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/agentstate"
	"github.com/PicketX/PicketXD/internal/hostowner"
	"github.com/PicketX/PicketXD/internal/hostpolicy"
	"github.com/PicketX/PicketXD/internal/hostrevoke"
	"github.com/PicketX/PicketXD/internal/markstate"
	"log/slog"
	"path/filepath"
	"time"
)

type Owner interface {
	RuntimeScope(context.Context) (hostowner.RuntimeScope, error)
	ApplyPolicy(context.Context, *hostpolicy.Policy, markstate.Range) (hostowner.ApplicationStatus, error)
	Verify(context.Context) (hostowner.Status, error)
	Status() hostowner.Status
	ApplicationStatus() hostowner.ApplicationStatus
	RevokeRecorded(context.Context) (hostrevoke.Result, error)
	HistoryStatus() hostowner.HistoryStatus
	Close() error
}
type Store interface {
	Snapshot(time.Time) (agentstate.Status, *configuration.Candidate, error)
	Reload(context.Context, string) error
	Run(context.Context, string, time.Duration, func(agentstate.Status) error) error
	Close() error
}
type Dependencies struct {
	OpenOwner func(context.Context, hostowner.Config) (Owner, error)
	NewStore  func(agentstate.Config) (Store, error)
	Emit      func(Status) error
	Logger    *slog.Logger
	Now       func() time.Time
}
type DesiredStatus struct {
	RemoteState   string    `json:"remoteState,omitempty"`
	Prepared      bool      `json:"prepared"`
	Generation    uint64    `json:"generation"`
	PolicyHash    string    `json:"policyHash"`
	AcceptedAt    time.Time `json:"acceptedAt"`
	LastAttemptAt time.Time `json:"lastAttemptAt"`
	LastError     string    `json:"lastError,omitempty"`
	Restored      bool      `json:"restored"`
	SameBoot      bool      `json:"sameBoot"`
	CacheFault    bool      `json:"cacheFault"`
	NextChange    time.Time `json:"nextChange"`
}
type Status struct {
	Strict               *StrictStatus               `json:"strict,omitempty"`
	History              hostowner.HistoryStatus     `json:"history"`
	At                   time.Time                   `json:"at"`
	Desired              DesiredStatus               `json:"desired"`
	Ownership            hostowner.Status            `json:"ownership"`
	Application          hostowner.ApplicationStatus `json:"application"`
	AppliedGeneration    uint64                      `json:"appliedGeneration"`
	AppliedPolicyHash    string                      `json:"appliedPolicyHash,omitempty"`
	EnforcementConfirmed bool                        `json:"enforcementConfirmed"`
	LastApplyError       string                      `json:"lastApplyError,omitempty"`
}

func desired(st agentstate.Status) DesiredStatus {
	return DesiredStatus{RemoteState: st.RemoteState, Prepared: st.Prepared, Generation: st.Generation, PolicyHash: st.PolicyHash, AcceptedAt: st.AcceptedAt, LastAttemptAt: st.LastAttemptAt, LastError: st.LastError, Restored: st.Restored, SameBoot: st.SameBoot, CacheFault: st.CacheFault, NextChange: st.Projection.NextChange}
}

// Run owns the resources it opens. An application/verification error is fatal;
// invalid publication reloads retain accepted intent and native deadlines.
func Run(ctx context.Context, c Config, d Dependencies) (err error) {
	c, err = c.normalized()
	if err != nil {
		return err
	}
	if d.Emit == nil {
		return fmt.Errorf("status consumer required")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.OpenOwner == nil {
		d.OpenOwner = func(ctx context.Context, c hostowner.Config) (Owner, error) { return hostowner.Open(ctx, c) }
	}
	if d.NewStore == nil {
		d.NewStore = func(c agentstate.Config) (Store, error) { return agentstate.New(c) }
	}
	owner, err := d.OpenOwner(ctx, hostowner.Config{StatePath: filepath.Join(c.StateDir, "owner.json"), ProviderID: c.ProviderID, AgentRef: c.AgentRef, RuntimeKey: c.runtimeKey()})
	if err != nil {
		return err
	}
	if owner == nil {
		return fmt.Errorf("owner factory returned nil")
	}
	defer func() { err = errors.Join(err, owner.Close()) }()
	runtime, err := owner.RuntimeScope(ctx)
	if err != nil {
		return err
	}
	if mode := owner.ApplicationStatus().Handling; mode != "" && mode != c.Handling {
		return fmt.Errorf("handling migration from %s to %s requires explicit deployment", mode, c.Handling)
	}
	facts, scope, err := c.facts(runtime)
	if err != nil {
		return err
	}
	store, err := d.NewStore(agentstate.Config{Path: filepath.Join(c.StateDir, "desired.json"), ProviderID: c.ProviderID, AgentRef: c.AgentRef, RuntimeKey: c.runtimeKey(), BootID: runtime.BootID, Facts: facts, Now: d.Now, Remote: c.Controller})
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("store factory returned nil")
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	var generation uint64
	var hash, lastReloadError string
	step := func(_ agentstate.Status) error {
		st, candidate, e := store.Snapshot(d.Now())
		if e != nil {
			return e
		}
		var applyErr error
		var previous *CleanupObservation
		if st.CacheFault {
			applyErr = fmt.Errorf("desired cache durability uncertain; reopen to recover")
		} else if candidate == nil {
			applyErr = fmt.Errorf("no accepted desired state")
		} else if generation != st.Generation || hash != st.PolicyHash {
			p, e := hostpolicy.CompileInput(candidate.Input(), scope)
			if e != nil {
				applyErr = e
			} else {
				// Clean against the retained image before a new permission can
				// preserve expired old flows. Initial marker-only startup has no
				// previous image; ordinary same-generation polls use Verify below.
				if old := owner.ApplicationStatus(); c.Handling == networkplan.Strict && old.Handling == networkplan.Strict && old.ImageDigest != "" {
					previous = &CleanupObservation{ImageDigest: old.ImageDigest}
					previous.Result, applyErr = owner.RevokeRecorded(ctx)
					previous.ObservedAt = d.Now().Round(0).UTC()
					if applyErr == nil && previous.Result.Remaining != 0 {
						applyErr = fmt.Errorf("previous strict cleanup did not confirm zero remaining")
					}
					if after := owner.ApplicationStatus(); applyErr == nil && (after.ImageDigest != old.ImageDigest || !after.EnforcementConfirmed || !owner.Status().OwnershipConfirmed) {
						applyErr = fmt.Errorf("previous strict image confirmation lost before replacement")
					}
					previous.Confirmed = applyErr == nil
					if applyErr != nil {
						applyErr = fmt.Errorf("cleanup retained strict image before replacement: %w", applyErr)
					}
				}
				if applyErr == nil {
					receipt, e := owner.ApplyPolicy(ctx, p, c.Marks)
					applyErr = e
					if e == nil {
						if !receipt.EnforcementConfirmed {
							applyErr = fmt.Errorf("application returned no live confirmation")
						} else {
							generation = st.Generation
							hash = st.PolicyHash
						}
					}
				}
			}
		} else {
			_, applyErr = owner.Verify(ctx)
		}
		var strict *StrictStatus
		if c.Handling == networkplan.Strict {
			strict = &StrictStatus{Enabled: true, PreviousCleanup: previous, Result: hostrevoke.Result{Remaining: -1}, ValidUntil: st.Projection.NextChange}
			if applyErr == nil {
				strict.Result, applyErr = owner.RevokeRecorded(ctx)
				strict.ObservedAt = d.Now().Round(0).UTC()
				strict.ImageDigest = owner.ApplicationStatus().ImageDigest
				if applyErr == nil && strict.Result.Remaining != 0 {
					applyErr = fmt.Errorf("recorded strict cleanup did not confirm zero remaining")
				}
				if applyErr == nil && !strict.ValidUntil.IsZero() && !strict.ObservedAt.Before(strict.ValidUntil) {
					applyErr = fmt.Errorf("membership boundary crossed during strict cleanup; reopen and retry")
				}
			}
		}
		// Acceptance can change while kernel IO is in progress. Label the retained
		// receipt against the latest coherent generation, never a stale callback.
		current, _, e := store.Snapshot(d.Now())
		if e != nil {
			return errors.Join(applyErr, e)
		}
		if strict != nil && applyErr == nil && !strict.ValidUntil.IsZero() && !current.At.Before(strict.ValidUntil) {
			applyErr = fmt.Errorf("membership boundary crossed before final strict status; reopen and retry")
		}
		report := Status{At: current.At, Desired: desired(current), Ownership: owner.Status(), Application: owner.ApplicationStatus(), History: owner.HistoryStatus(), AppliedGeneration: generation, AppliedPolicyHash: hash}
		report.EnforcementConfirmed = applyErr == nil && !current.CacheFault && current.Prepared && generation == current.Generation && hash == current.PolicyHash && report.Application.EnforcementConfirmed && report.Ownership.OwnershipConfirmed
		if strict != nil {
			strict.CleanupConfirmed = report.EnforcementConfirmed
			report.Strict = strict
		}
		if applyErr != nil {
			report.LastApplyError = applyErr.Error()
		}
		if d.Logger != nil {
			if current.LastError != "" && current.LastError != lastReloadError {
				d.Logger.WarnContext(ctx, "publication rejected; retaining accepted desired state", "component", "static-enforce", "generation", current.Generation, "error", current.LastError)
			}
			if applyErr != nil {
				d.Logger.ErrorContext(ctx, "host enforcement stopped; installed rules retained", "generation", current.Generation, "error", applyErr)
			}
		}
		lastReloadError = current.LastError
		emitErr := d.Emit(report)
		if applyErr != nil || emitErr != nil {
			return errors.Join(applyErr, emitErr)
		}
		if c.Once && !report.EnforcementConfirmed {
			return fmt.Errorf("accepted desired state changed during application; retry")
		}
		return nil
	}
	if c.Once {
		loadErr := store.Reload(ctx, c.PolicyPath)
		st, _, e := store.Snapshot(d.Now())
		if e != nil {
			return errors.Join(loadErr, e)
		}
		if e = step(st); e != nil {
			return errors.Join(loadErr, e)
		}
		return loadErr
	}
	err = store.Run(ctx, c.PolicyPath, c.Poll, step)
	if ctx.Err() != nil && err == ctx.Err() {
		err = nil
	}
	return err
}
