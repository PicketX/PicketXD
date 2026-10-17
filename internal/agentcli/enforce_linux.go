// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
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
	var handling string
	var systemdNotify bool
	cmd := &cobra.Command{Use: "enforce", Short: "Apply and recover static host INPUT policy in this network namespace", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		notifySocket, e := enforceNotifySocket(systemdNotify, c.Once)
		if e != nil {
			return e
		}
		bounds, e := networkplan.ParseDestinations(destinations)
		if e != nil {
			return fmt.Errorf("destination bounds: %w", e)
		}
		c.Destinations = bounds
		c.Handling = networkplan.HandlingMode(handling)
		return hostagent.Run(cmd.Context(), c, hostagent.Dependencies{Logger: logger, Emit: newEnforceEmitter(cmd.OutOrStdout(), notifySocket)})
	}}
	f := cmd.Flags()
	f.StringVar(&handling, "handling", "graceful", "Explicit host handling: graceful or strict; mode migration requires compatible deployment")
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
	f.BoolVar(&systemdNotify, "systemd-notify", false, "Notify systemd only after confirmed current policy; requires NOTIFY_SOCKET and long-running mode")
	for _, name := range []string{"policy", "state-dir", "provider", "agent", "destination", "mark-first", "mark-last"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}
