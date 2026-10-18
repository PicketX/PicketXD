# Controller Provider design

The user has authorized autonomous design, execution, review, push and expected-head merge.
This increment connects the existing Controller stream to the existing host executor; it
is architectural because it adds a provider trust boundary. The selected approach adapts
complete snapshots to the existing configuration compiler and execution driver. A separate
executor would duplicate strict cleanup/recovery; trusting remote namespace IDs would let
publisher facts impersonate local observations. Both alternatives are rejected.

Exactly one source is selected: static YAML or Controller address/token. Controller authority,
Agent, profile, handling, destination bounds, mark reservation and transport configuration
are explicit local settings. Remote host-local/original-destination requests must fit these
bounds; remote context/network-space identities are never treated as local kernel facts.
One remote context maps to the locally held context. Unsupported capabilities or ambiguous
inventory is rejected as a whole, including dormant contributions. A full empty snapshot
withdraws protection. Removing grants alone retains protected entries with default deny.

The remote wire envelope is verified, revision regressions/equivocation rejected, and the
raw accepted snapshot plus derived policy are atomically stored in a private journal.
The cache is bound to provider type, authority, Agent, endpoint/TLS settings and local scope.
No static/remote cache adoption. Restore verifies bytes and rederives policy against current
local facts. Invalid frames and disconnection preserve accepted intent, never extend expiry.
Initial absence of LKG waits for valid intent; it does not assert protected policy readiness.

The existing owner/mark/application/strict-cleanup path remains the sole executor. Stream
acceptance is serialized with application and reports RECEIVED then APPLIED only after clean
current proof; rejection/application failure reports FAILED. A send failure reconnects without
modifying deadlines. Reports remain authenticated Agent assertions, not Controller-independent
verification. Static readiness semantics remain unchanged. Optional TLS validates certificates;
plaintext must be explicitly selected. Token files are private, bounded regular files.

Tests cover malformed/scoped snapshots, independent dual-stack deadlines, deny/union,
withdrawal, stale/equivocating revisions, foreign/corrupt cache, restart/outage, real TCP/TLS,
actual CLI and isolated Linux execution. No mocked kernel acceptance. No management or L7
completion claim in this increment.
