# M2C1: Durable host ownership

> English (default) | [简体中文](2026-10-16-host-ownership-design.zh-CN.md)

The user authorized continued implementation after all outstanding PRs were merged. This focused slice resolves the ownership prerequisite in the [production brief](2026-10-16-production-l4-readiness.md). It introduces actual kernel ownership, not traffic enforcement. Implement inline with one final independent review.

`internal/hostowner.Open(ctx, Config)` acquires a namespace-wide lock, restores a private ownership journal, inspects the live table, durably records an attempt, exclusively creates a marker-only table if appropriate, reads it back, and durably confirms ownership. `Session.Close` releases descriptors/locks without deleting the table. `Session.Verify(ctx)` freshly observes ownership; a prior successful check is not a continuing assertion about kernel state. `Status` always reports `enforcementConfirmed=false`.

Config fixes StatePath, ProviderID, AgentRef and RuntimeKey. Bind the session to a held `/proc/thread-self/ns/net` descriptor and boot ID, with device/inode metadata. An abstract Unix datagram socket binds the singleton to the network namespace even across different mount namespaces. Runtime file lock names also depend on boot and namespace, not the selected state path; production lock root is fixed `/run/picketx`. Also lock the selected journal itself so different namespaces cannot race to publish the same state file. Require private current-owner regular files/directories, bounded nonblocking reads, no symlink traversal in managed directory components, canonical JSON and complete directory/file synchronization. Reuse the existing durability semantics, without changing the desired-state journal format.

The kernel marker is one counter-only rule in an unhooked `picketx__owner` chain in `inet picketx`. UserData encodes protocol version, random installation ID, scope hash and random epoch token (all bounded canonical ASCII). Scope hashes provider/Agent/runtime identity; epoch tokens bind the actual boot/namespace. Inspect the complete table: exactly this chain/rule, no hooks, sets, objects or flowtables, default table flags. Unknown additional structure is an error. Markers detect collisions under trusted NET_ADMIN; they are not authentication against other privileged managers. This slice allocates no conntrack marks, installs no hooks/filters and deletes no kernel objects.

The journal contains version, explicit provider/Agent/runtime identity, installation ID, boot ID, namespace device/inode, epoch token and phase (`pending` or `confirmed`). Validate all values and canonical encoding before using them. Persist pending before dispatch; save confirmed only after live structural readback. Saving with uncertain durability stops the operation; it must be reopened. Kernel operations never precede successful pending persistence. A create/ACK/readback failure leaves recoverable pending state; no rollback deletion.

| Recovered journal | Live table | Result |
| --- | --- | --- |
| Absent | Absent | Generate and durably persist pending, exclusive create, readback, confirm |
| Absent | Present | Refuse; never adopt by table name |
| Pending, matching scope/namespace | Absent | Retry the same epoch; persist already exists before dispatch |
| Pending, matching scope/namespace | Exact matching marker | Confirm ownership from fresh readback, then persist confirmed |
| Confirmed, matching scope/namespace | Exact matching marker | Reobserve, no table rewrite; disk confirmation alone is insufficient |
| Confirmed, same boot/namespace | Absent | Refuse drift; no silent recreation |
| Any | Foreign marker or unsupported structure | Refuse without mutation |
| Any, same boot but different namespace | Any | Refuse identity mismatch |
| Any, new boot | Absent | Keep installation ID, generate a fresh epoch bound to current namespace, persist pending and claim only an empty marker table; no policy or applied-state recovery is asserted |
| Any, new boot | Present | Refuse; old-boot identity is not adoption authority |

All inspection dumps must complete successfully. Create the namespace pin, abstract lock and lasting nftables connection on one locked OS thread; the sockets stay bound to that namespace without calling setns or requiring CAP_SYS_ADMIN. Use exclusive `CreateTable` so a table appearing between inspection and dispatch aborts the whole transaction. Close returns an error only for resource-release failures, never claims to remove protection. Cancellation stops before each new phase; an in-flight Netlink operation is not promised to be interruptible. Two table observations around structural inspection reduce ordinary drift ambiguity; no linearizability against an adversarial privileged writer is promised.

Acceptance: real filesystem lock/durability/reopen tests; failure injection at pending-save, dispatch, readback and confirmed-save; foreign/same-name/alternate-path/concurrent callers; new-boot distinction; bounded private-file rejection; independent-process lock release after forced exit; real namespace/table readback and exclusive-create races. Retain all 43 existing required kernel scenarios and add ownership cases to the required evidence gate. `prepare` remains unchanged. Future enforcement must define a versioned structural contract before adding chains, mark allocation or cleanup.
