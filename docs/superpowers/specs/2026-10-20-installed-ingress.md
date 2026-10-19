# Installed ingress conjunction

Self-reviewed focused integration-interface design, 2026-10-19T23:42Z.
Core main adfa1b111e719b606222f6643c7343afa98d3f03 supplies shipped
`picketx nginx-init`; Agent main c0eb1b8baf23d45e3db545566d8c1ad1638a413d
still consumes a Core source-tree Nginx template in native acceptance.

## Contract and context map

- `integration/controller-install.mjs`: verify exact bundle/source and initialize
  private instance; expose installed CLI/token **paths**, never secret arguments.
- `integration/installed-ingress.mjs` (new): invoke that actual CLI with empty
  PATH, exclusive private output directories and three fixed route UIDs; inspect
  regular0600 files/0700 directory and hash them. Return private include paths
  separately from an allowlisted public receipt.
- `integration/{console-kernel,gitea-application}.mjs`: existing ingress fixture
  owns listeners/TLS/proxy/health/keepalive. Include generated location-context
  and server-context files. Preserve primary, denied private and Gitea mappings;
  caller registry remains only route-v1 and route-gitea-v1. No template reads.
- `integration/{controller-install,console-evidence}.test.mjs`: independent
  literal unit fixtures and strict old/malformed/secret/route/hash rejection.
- `.github/workflows/validation.yml`, `scripts/test-controller-service.py`:
  pin new accepted Core commit/tree, preserve all old gates and four archives.
- `deploy/controller/README*.md`, delivery checkpoint and validation record:
  explain shipped CLI operation and actual evidence, no new runtime/API/UI claim.

All existing real browser/HTTP/Agent/kernel/Gitea/service/backup tests remain
mandatory. Generated includes alone neither enroll a caller nor create a Grant.
Keep GPL/Apache boundaries and ingress-owned TLS, no production behavior changes.
Public evidence must require actual installed CLI hash, empty PATH, exact three
route receipts, file hashes/privacy and two tested server config hashes; do not
publish include contents, token paths or Nginx expanded configuration.

## Acceptance and boundaries

Watch old schema accept a missing ingress receipt (RED), then require it with
mutations (GREEN). Run the real old CLI missing-command probe, then the new
verified bundle successfully. Native CI must execute generated includes through
real Nginx and preserve every previous positive/negative control. Verify complete
local suites (record inherited five Unixgram EPERM), full native jobs/raw logs,
four actual archives/source identities, one independent full-branch review,
expected-head merge and main CI. No full-product completion claim from this slice.

Ruling: explicit user authorization waives routine design/plan/push/merge pauses;
integration only, not a new ingress product. Cost if wrong: acceptance/configuration
drift; actual native conjunction and operator review remain mandatory.
