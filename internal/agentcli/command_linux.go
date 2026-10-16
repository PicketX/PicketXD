// SPDX-License-Identifier: GPL-3.0-or-later
package agentcli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/PicketX/PicketX/pkg/configuration"
	"github.com/PicketX/PicketX/pkg/networkplan"
	"github.com/PicketX/PicketXD/internal/agentstate"
	"github.com/spf13/cobra"
)

func New(out io.Writer, logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{Use: "picketxd", Short: "PicketXD static host policy preparation and enforcement", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(out)
	var policy, state, provider, agent, profile string
	var destinations []string
	var once bool
	var poll time.Duration
	prepare := &cobra.Command{Use: "prepare", Short: "Prepare and recover static desired state; does not enforce traffic", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if poll <= 0 {
			return fmt.Errorf("poll must be positive")
		}
		bounds, e := networkplan.ParseDestinations(destinations)
		if e != nil || len(bounds) == 0 {
			return fmt.Errorf("explicit destination IP/CIDR bounds are required")
		}
		boot, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
		if e != nil {
			return e
		}
		bootID := strings.TrimSpace(string(boot))
		ns, e := os.Readlink("/proc/self/ns/net")
		if e != nil {
			return e
		}
		space := bootID + "/" + ns
		keyBytes, e := json.Marshal(struct {
			Agent, Profile string
			Destinations   any
		}{agent, profile, bounds})
		if e != nil {
			return e
		}
		runtimeKey := fmt.Sprintf("%x", sha256.Sum256(keyBytes))
		facts := configuration.Facts{AuthorityID: provider, Agents: []networkplan.Agent{{Name: agent, Capabilities: &networkplan.CapabilityProfile{Classes: []networkplan.TrafficClass{networkplan.HostLocal}, Protocols: []networkplan.Protocol{networkplan.TCP, networkplan.UDP}, Families: []networkplan.Family{networkplan.IPv4, networkplan.IPv6}, Handling: []networkplan.HandlingMode{networkplan.Graceful}, OriginalDestination: true}}}, Contexts: []networkplan.ExecutionContext{{AgentRef: agent, ProfileRef: profile, ID: space + "/" + runtimeKey, NetworkSpaceID: space, Class: networkplan.HostLocal, Stage: networkplan.OriginalDestination, Destinations: bounds, Handling: networkplan.Graceful}}}
		store, e := agentstate.New(agentstate.Config{Path: state, ProviderID: provider, AgentRef: agent, RuntimeKey: runtimeKey, BootID: bootID, Facts: facts})
		if e != nil {
			return e
		}
		defer store.Close()
		enc := json.NewEncoder(cmd.OutOrStdout())
		lastError := ""
		emit := func(st agentstate.Status) error {
			if logger != nil && st.LastError != "" && st.LastError != lastError {
				logger.WarnContext(cmd.Context(), "publication rejected; retaining accepted desired state", "component", "static-prepare", "generation", st.Generation, "error", st.LastError)
			}
			lastError = st.LastError
			return enc.Encode(st)
		}
		if once {
			loadErr := store.Reload(cmd.Context(), policy)
			if loadErr != nil && store.Current() == nil {
				return loadErr
			}
			st, e := store.Status(time.Now())
			if e != nil {
				return e
			}
			if e = emit(st); e != nil {
				return e
			}
			return loadErr
		}
		return store.Run(cmd.Context(), policy, poll, emit)
	}}
	flags := prepare.Flags()
	flags.StringVar(&policy, "policy", "", "Complete atomic YAML publication")
	flags.StringVar(&state, "state", "", "Private accepted desired-state cache path")
	flags.StringVar(&provider, "provider", "", "Stable static authority identity")
	flags.StringVar(&agent, "agent", "", "Local static Agent name")
	flags.StringSliceVar(&destinations, "destination", nil, "Explicit destination IP/CIDR bounds (repeatable)")
	flags.StringVar(&profile, "profile", "host", "Local host preparation profile")
	flags.DurationVar(&poll, "poll", 5*time.Second, "Publication polling interval")
	flags.BoolVar(&once, "once", false, "Prepare once, report and exit")
	for _, name := range []string{"policy", "state", "provider", "agent", "destination"} {
		_ = prepare.MarkFlagRequired(name)
	}
	root.AddCommand(prepare, enforceCommand(logger))
	return root
}
