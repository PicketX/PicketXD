// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
	"encoding/json"
	"fmt"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/hostagent"
	"github.com/spf13/cobra"
	"log/slog"
	"time"
)

func enforceCommand(logger *slog.Logger) *cobra.Command {
	var c hostagent.Config
	var destinations []string
	cmd := &cobra.Command{Use: "enforce", Short: "Apply and recover static host INPUT policy in this network namespace", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		bounds, e := networkplan.ParseDestinations(destinations)
		if e != nil {
			return fmt.Errorf("destination bounds: %w", e)
		}
		c.Destinations = bounds
		encoder := json.NewEncoder(cmd.OutOrStdout())
		return hostagent.Run(cmd.Context(), c, hostagent.Dependencies{Logger: logger, Emit: func(st hostagent.Status) error { return encoder.Encode(st) }})
	}}
	f := cmd.Flags()
	f.StringVar(&c.PolicyPath, "policy", "", "Complete atomic YAML publication")
	f.StringVar(&c.StateDir, "state-dir", "", "Absolute private owner/desired state directory")
	f.StringVar(&c.ProviderID, "provider", "", "Stable static authority identity")
	f.StringVar(&c.AgentRef, "agent", "", "Local static Agent name")
	f.StringVar(&c.Profile, "profile", "host", "Local host profile")
	f.StringSliceVar(&destinations, "destination", nil, "Explicit destination IP/CIDR bounds (repeatable)")
	f.Uint32Var(&c.Marks.First, "mark-first", 0, "First full operator-reserved conntrack mark (decimal or 0x hex)")
	f.Uint32Var(&c.Marks.Last, "mark-last", 0, "Last full reserved mark, inclusive; at most 4096 values")
	f.DurationVar(&c.Poll, "poll", 5*time.Second, "Publication polling and live verification interval")
	f.BoolVar(&c.Once, "once", false, "Apply/recover once and exit, retaining installed filtering")
	for _, name := range []string{"policy", "state-dir", "provider", "agent", "destination", "mark-first", "mark-last"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}
