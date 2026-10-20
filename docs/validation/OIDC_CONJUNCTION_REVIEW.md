# Installed OIDC conjunction review

One independent fresh-context whole-branch review of 0255d4d..ad5ef7a found
0 Critical, 1 Important, 0 Minor. The Important finding: abrupt native child
termination bypasses child finally, leaving its sibling temporary credentials
and Gitea NSS label outside the wrapper's ownership.

Root correction: a separate parent-owned native root0711 contains every child
TMPDIR and an isolated private HOME0700/NSS. Provider remains separately0700.
After checked process-group termination, the wrapper removes this whole tree,
including both fixture CAs, before its success receipt. Native child SIGTERM and
SIGKILL regressions fail with missing ownership helper, then pass after the fix;
actual child credential and trust-storage residue is removed. Filesystem modes
remain checked. The sandbox rejects setuid subprocess execution, so that local
probe is not counted; actual native Gitea must independently confirm traversal.

Native candidate39385136548 independently failed at oidc-secure-session before
any human requests: URL-filtered cookies queried site root, omitting the actual
/console/ cookie. Query the installed console path. Controlled cookie contract
regression was RED, then GREEN; only actual normal-TLS browser CI establishes
real login success. No production authentication or Cookie policy was changed.

Full261 Node tests pass without skip; original review independently passed258.
Reviewer reproduced the five inherited Go Unixgram EPERM failures and no other
package failures; local normal/race were already recorded, not called native pass.
Exact final native CI/raw/four actual archives/source/main verification remains
required. Earlier failed candidates are diagnosis, never final acceptance.

## Rulings and limits
- Explicit existing autonomous authorization covers routine integration; all
  evidence gates remain. Wrong: configuration drift or unverified release.
- Use a separate native root and isolated trust storage, preserving provider0700
  and Gitea traversal. Wrong: private residue or broken real application access.
- Reviewer did not certify actual CI or archives: root must independently verify
  raw logs, source, binaries and four archives. Wrong: stale or incomplete proof.
- Production IdP operation, MFA/HA, arbitrary providers and new IdP outage behavior
  remain outside this acceptance. Wrong: unsupported deployment expectations.
- Unchanged product/kernel internals were not freshly audited; existing actual
  native/kernel/service gates remain mandatory. Wrong: inherited defects.

No deferred Minor findings from this review. Final immutable PR28 receipt
supersedes pending status here; this document does not declare overall completion.
