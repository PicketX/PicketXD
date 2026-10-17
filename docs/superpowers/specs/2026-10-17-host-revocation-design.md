# M2C6: scoped production host conntrack cleanup

> English (default) | [简体中文](2026-10-17-host-revocation-design.zh-CN.md)

## Intent and boundary

Provide the production cleanup engine needed by strict L4, following M2C5. This increment adds typed complete conntrack scans, immutable ownership/current-permission selection, exact ID-guarded deletion and a held Session entry point. The command remains explicitly graceful: no automatic cleanup, strict-mode configuration, durable cumulative image history, process-loss strict teardown or whole-Agent strict confirmation is claimed. User approved continued staged delivery and this scan/select/delete increment; execute inline and merge only passing reviewed work.

## Contract

`hostrevoke.New(history []hostrules.Image, current hostrules.Image, marks markstate.State) (*Plan,error)` validates and independently copies all images, with at most 32 images / 32 MiB cumulative JSON. All images must have equal authority/execution context; every target identity/mark must match the durable ledger, including retired marks. Current target marks must be active. Historical image inputs are caller-supplied candidate ownership evidence, not inferred emission receipts or a completeness guarantee. The Session always adds its actual confirmed image to history; callers retain older images for broader cleanup. No arbitrary mark, endpoint, current image or persisted confirmation flag can replace the held Session's live current image.

Ownership requires exact known nonzero mark, original destination/protocol/port inside that image's target, and a historical allowing source (absolute historical expiry is ignored because graceful established flows survive it). Current permission is the union across all current physical targets, with deny precedence, absolute NotBefore inclusive and ExpiresAt exclusive. Current allowances preserve old-mark flows too. Unmarked, wrong-source, wrong-endpoint and unrelated-mark flows remain untouched. Selection uses original tuples including DNAT, never reply destination as the policy target.

Typed TCP/UDP `Flow` retains family, mark, zone, opaque four-byte kernel ID and complete original/reply tuples. Strict decoder validates ordinary aligned NLA envelopes, duplicate and missing identity fields, widths, family/protocol consistency and scalar/container shape. Unknown top-level metadata is structurally checked and ignored; non-TCP/UDP flows are excluded. Direction-specific tuple zones are unsupported and fail the scan, never silently use zone zero. Both family dumps complete before the first delete; any partial/interrupted/malformed dump produces no selected output. Limit native dumps to 64 MiB per family and typed snapshots to 262144 flows. Existing strict Netlink datagram/DONE/sequence/PID/error checks remain in force.

`Run(ctx, plan, Effects{Confirm,Dump,Delete,Now}) (Result,error)` requires all operations. Confirm reobserves the actual current image before scanning, before each deletion and after confirmation scans. Complete both families, select once, then refresh the clock and current eligibility before each exact deletion; reject backward clocks and a crossed current-membership boundary rather than silently declare success. Reaching a future boundary is a retryable error; effects are not an atomic transaction with the wall clock, so this is not process-free strict scheduling. No dynamic desired update may run concurrently with cleanup. Re-dump both families and report `Remaining=-1` on incomplete confirmation; any remaining matching flow or delete error is failure. ENOENT is counted separately as already absent, never as an acknowledged deletion. No whole-table flush or broad kernel mark filter exists.

`Session.RevokeWithdrawn(ctx,history) (hostrevoke.Result,error)` holds its existing mutex, requires a confirmed current application and mark ledger, builds the plan from independently cloned values, revalidates held identity and verifies full live image for every Confirm. It never writes the journal or nft rules. Backend uses the existing held strict Netlink socket; exact delete contains original tuple, zone and opaque CTA_ID, so kernel ID mismatch refuses a replacement entry. Reply/mark equality is checked by the full observation; Linux DELETE itself checks original tuple/zone/ID, not reply/mark. Privileged writer exclusivity remains required. Any cleanup error clears Session proof/faults the session for reopen; success provides scoped counts only, not a persisted strict receipt.

## Context map

| File | Purpose/change |
| --- | --- |
| `internal/hostrevoke/{plan,run}.go`, tests | New immutable production selector and confirm/scan/delete/reconfirm protocol; imports hostrules/markstate/Core, never validation fixtures. |
| `internal/hostrevoke/wire_linux.go`, tests | Strict typed decoder and exact-delete encoder; existing mdlayher Netlink types/UAPI. |
| `internal/hostowner/cleanup_linux.go`, tests | Held-session API, reader/deleter adapters using strict socket and existing ACK transaction helper. |
| `internal/hostowner/cleanup_integration_test.go` | Actual owned engine, dual-stack TCP/UDP, preservation, DNAT, zones/ID mismatch, drift and injected partial native failures. |
| `scripts/test-kernel.sh`, report | Existing owner stream extended with required cleanup cases, all 71 previous gates retained. |
| bilingual README/validation/spec/plan | Exact observed evidence and staged boundary, no CLI strict claim. |

Dependencies: immutable hostrules.Image and markstate.State; held Session.ApplyPolicy/Verify/strictNetlink; existing admission test namespace/server helpers and required report. No module/journal/config migration. Reference patterns: existing native strict dump, image ACK writer, validation ctbackend algorithm (concept only; no production import).

## Risks and acceptance

No public manifest/CLI change, schema migration or dependency change. Destructive scope is explicitly constrained by validated images + durable marks + live confirmation; malformed evidence fails before deletion. Unit RED→GREEN must exercise cross-family atomic preflight, exact scopes, UID-independent union/deny/lifetimes, immutable input, cancellation, confirmation failure, partial delete, remaining flows, ID/zone/endian/NAT and malformed wire. Full unit/race/vet/build/static smoke and integration compile pass. One fresh whole-branch independent review; Critical/Important fixes observed RED→GREEN. Real disposable-namespace CI must pass all prior gates plus new required native cleanup cases; download raw evidence, verify exact final PR/main trees, binary/module metadata and artifacts before completing.

Kernel contract source: Linux v6.17 `net/netfilter/nf_conntrack_netlink.c` ctnetlink_del_conntrack validates CTA_ID against nf_ct_get_id before nf_ct_delete; ID bytes are opaque. No claim that DELETE atomically compares mark/reply/policy or that cleanup alone prevents TCP resurrection under a graceful admission image. Strict admission/lifecycle is the subsequent integration.
