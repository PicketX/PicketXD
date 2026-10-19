# Managed DB human / native conjunction review

One fresh independent read-only whole-branch review a60ca58d784f20eb9510817070ae2decec3031a4
..cc54c3db20b9b600a41ab4e3370bcfeb718012c1 covered all11 files, spec/plan/ledger
and adjacent pinned Core account/session/UI contracts. Reviewer independently ran
Node89/89 and diff checks, and inspected actual downloaded31-case/13-confirmation
report. No Critical. Two Important retained by root, one Minor explicitly deferred.
No fabricated GitHub approval, no re-review or delegated secondary reviewer.

## Single root fix pass

R1: The first disabled /me401 destroys the SCS server session. Restoring its cookie
bytes after re-enable falsely passes even for a generation-blind resolver. Reviewer
demonstrated this with an explicitly labeled in-memory counterexample (not native
evidence); root inspected Browser.principal's Destroy path. Four independent-session
evidence regressions observed RED→GREEN. Native workflow now creates two distinct
password sessions, verifies both200 before editing, probes one while disabled and
leaves the second wholly unprobed until after re-enable401. Cookie values are
compared only in memory, never serialized. Fresh password login still binds same ID.

R2: Publication revision/traffic alone could miss a persisted Grant deadline edit,
especially IPv4 which is explicitly revoked before its deadline. Six full-Grant
readback regressions (missing receipt, expiry extension, source/scope changes)
observed RED→GREEN. After each account edit, real API readback of both complete
Grants must equal original approval snapshots, including exact absolute deadlines.
Full Node99/99 green. The first native green is not final generation-safety proof;
patched exact-head full native CI and main verification remain mandatory in PR20.

## Deferred Minor

R3: Whole public directory/me responses are retained; validator tolerates extra
fields. Pinned Core DTOs and actual report are credential-free, but a future API
regression could emit unexpected sensitive fields. Explicit allowed-field projection
and adversarial serialization tests are bounded follow-up; not an observed leak.
Root retains Minor severity. Core PR33 superseded failed-GET warning and PR32 Chinese
UTF8 byte-feedback Minors remain deferred, outside this Agent test-only increment.

## Exhaustive root rulings on reviewer boundaries

- Exact final/native/main CI, digests, source identity and screenshots: root checks
  original steps/raw logs/downloaded bytes/375+1440 screenshots and safe merge parents.
  Cost if wrong: false acceptance of a different source; merge remains gated.
- Native kernel/systemd/full Go/race: root requires unrestricted Linux originals;
  local5 known Unixgram EPERM failures remain honest. Cost: compatibility limited to
  actual recorded kernel/toolchain, not every Linux environment.
- Broader Core account/session correctness: retain accepted Core31c7ac7 and inspect
  used contracts, not re-review whole Core. Cost: relies on its prior tests/review.
- Existing two Core UI Minors: defer unchanged. Cost: conservative obsolete read
  warning and generic Chinese byte-limit rejection remain possible.
- OIDC/PostgreSQL/HA/GiteaHTTPS+SSH/productionTLS/deployment/release: no new claim;
  scope remains local DB humans and isolated plaintext ingress. Cost: those production
  and first Gitea acceptance items still need their own increments.
- Password reset/role edit/approver disable: this increment proves requester
  disable/re-enable; other account operations retain Core's existing tests.
  Cost: no new combined native evidence for those operations.
- Coherent malicious report fabrication: schema is not execution attestation;
  source/raw-log/archive checks supply evidence provenance. Cost: assumes CI and
  pinned binaries, not arbitrary hostile runners, are trusted.
- Historical91 required native gates: original unchanged count; root checks actual
  report112 cases and9systemd. Cost: do not conflate required gate IDs with case count.

Other execution rulings: preserve already isolated host-owned branch/worktree
(cost: manual local/remote tree identity checks); retain existing UI visual system
and apply UX guidance to real feedback/label-driven/mobile acceptance only (cost:
no new visual redesign claim). English and Chinese validation guides synchronized.

中文：一次全分支独立评审发现2个验收证据阻塞，均由作者在一次RED→GREEN修复中
补齐：未触碰的独立旧会话及每次账户变更后完整Grant/期限回读。1个未来敏感字段
序列化加固Minor明确延后；最终原生CI、摘要、源码树、精确合并及主线仍须核验。
