# Installed single-node Controller and Agent

> English (default) | [简体中文](README.zh-CN.md)

Use the Apache Core Controller bundle separately from the GPL Agent. Core pinned
93024b229c072ce1db8512323c3d7fa35e3df74e supplies private initialization and static
binaries/Web/runbooks/notices. Keep its LICENSE/notices/manifest/SOURCE together.
This guide does not publish a GPL Agent binary/source distribution or configure
your host automatically. Existing ingress owns TLS/forwarding; never expose the
loopback plaintext Agent or local decision listener to untrusted networks.

## Pair an existing host-local Agent

1. Follow the bundled Controller runbook to provision its unprivileged user/private
   parent, deploy the bundle and initialize a **new** instance. Select the actual
   browser origin and Agent ref with `--agent-ref`; `--route-id` scopes only the
   initial ingress credential, not a default HTTPRoute or Grant. Do not rerun init
   over a restore, retained instance or failed partial initialization.
2. Preserve generated source mode `database` and management users mode `database`.
   No policy/facts file polling may overwrite API-managed state. Bootstrap key,
   Agent token and ingress token have independent audiences and0600 files; protect
   their parent0700. Create human accounts via Accounts; keep bootstrap recovery
   access private. Native application passwords/tokens/SSH keys remain separate.
3. Install a separately built/verified static Agent executable, preserving its GPL
   source/license obligations. On the same trusted host, copy only `agent-token`
   to a root-owned private Agent path such as `/etc/picketx/agent-token` (0600).
   Never pass its value in arguments, environment, URL, logs or a browser form.
4. Record the generated `agent.authority` from controller.yaml and use that exact
   public value as Agent `--provider`. Match `--agent` to the enrolled ref. Reserve
   the complete mark range exclusively, set explicit dual-stack destination bounds,
   private persistent state/lock paths, host profile and chosen handling. For a
   loopback default Controller, use `--controller 127.0.0.1:9090`,
   `--agent-token-file /etc/picketx/agent-token`, `--controller-plaintext`; never also
   supply `--policy`. Remote transport instead needs an explicitly configured
   normal verified server TLS listener or a trusted tunnel; no insecure TLS bypass.
5. Configure actual deployment facts/capabilities and Binding scope through the
   existing managed Desired configuration/Resources workflow, using the generated
   authority and matching Agent/profile/destination bounds. Then configure Resource,
   HTTPRoute, independent approval/Grants and the existing trusted ingress adapter.
   A declarative network-space ID is not a local namespace identity.
6. Review the [systemd service guide](../systemd/README.md). Its shipped unit is a
   static-policy demonstration, **not** ready-made Controller enrollment. Make an
   operator-owned copy and replace its ExecStart with the reviewed Controller-mode
   arguments above plus the existing required state/provider/Agent/destination/mark
   settings. Preserve notify/sandbox/privilege restrictions; inspect before enabling.
   Do not install a sample mark range or destination bound blindly.

Before starting ingress, render its private config with only its own ingress key.
The initial caller registry authorizes one exact route UID. If a second actual
HTTPRoute (e.g. Gitea) is needed, explicitly edit that caller's allowed routes in
the private `callers.json` registry or provision a separate independently scoped
caller. Never reuse bootstrap/Agent tokens or broaden to every route. This is
credential enrollment, not an approval or proof of enforcement.

## Recovery and verification

Keep Controller SQLite state (including WAL/SHM as required by its stopped backup
procedure), generated authority/config and private registries/keys together. Keep
Agent owner/desired/LKG state and locks at the identical protected paths. Never
delete/rename retained state, change provider identity or rerun init to force a
restart through a rejection. Compatible migrations/rollback require separate review.

For planned process restart, stop Agent and Controller cleanly, preserve their
state, then start Controller and Agent with the identical reviewed settings. Old
in-memory browser sessions become invalid; sign in again. Inspect fresh desired/
applied revision and policy hash, enforcementConfirmed and the actual current gRPC
report, not just service-active or an old line. Confirm dual-stack real application
requests; later expiry/revocation must still deny. Account disable does not revoke
independent source Grants; revoke them explicitly.

Native CI rebuilds the actual Core bundle, tar/re-extracts, verifies every file,
relocates it and runs its generated config. Actual Controller and Agent run with
empty PATH/unrelated cwd, then restart with active Grants and unchanged business
views; require fresh post-restart Agent/Controller receipts and real IPv4/IPv6
requests before continuing every existing human/Gitea/revocation/expiry case. The
test driver, Nginx and Gitea use their own dependencies. Public evidence retains
source/binary/manifest hashes, redacted logs and complete Agent status history.

This is process recovery inside disposable Linux namespaces, **not a host reboot,
production systemd Controller installation, public TLS setup, backup/restore
execution or complete release acceptance**. Existing Agent systemd/kernel evidence
is separate. Graceful SSH denial concerns new flows only, not established SSH.
