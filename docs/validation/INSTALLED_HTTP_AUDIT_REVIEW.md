# Installed HTTP audit independent review

One fresh read-only whole-branch review, BASE665c9a4c2c7134543942ed769e6ba12814c5d825
through localdc5643ff908bd83c01cf9203f76ede893d3e89d3 (published72164f581fcf26b46dfcf5ce8a28d9d729c61d4a,
same tree61b0cf781a814433816625347a4a1d4b9993c030). Verdict With fixes:
0Critical/1Important/0Minor. No nested reviewer or second review.

Root verified the Important finding: swapping receipt.grants plus allowed API and
DOM contribution lists together passed the old gate. This was self-referential
public proof validation, not fabricated native requests. ONE correction pass
adds independent original Grant-create response UID/revision plus known family/
source in report.grantIssuances. Exact two sequential issuance revisions correlate
to existing grant4/grant6 applied cases and restored revision. Receipt IDs must
match that independent mapping. Seven regressions (coherent family swap, unrelated
UID, missing issuance, wrong issuance revision/source/case and private extra field)
all first failed Missing expected exception, then passed.199targeted tests pass.
The inherited stale-generation unit mutation initially became a no-op when an
extra revision1 confirmation was prepended; retaining the original first revision2
and appending revision1 preserved its real rejection. No old assertion was removed.
Complete green suite and final native/source/archive/main acceptance still required.

## Rulings and costs

1. Actual privileged Linux/archive/exact-head/main acceptance remains root-owned
   after this review, never inferred from192reviewer unit passes. Cost if wrong:
   untested native behavior could ship; mandatory gates prevent acceptance here.
2. Five inherited local Unixgram EPERM failures remain an environment limitation,
   not a regression or green native proof. Cost: local runtime cannot certify those
   paths; actual Linux ordinary/race gates remain mandatory.
3. Histories beyond bounded fixture paging fail closed; retain the existing oldest-
   first/200-row UI rather than adding search/export/redesign. Cost: this bounded
   fixture will fail instead of proving arbitrarily large histories.
4. Pinned visible DOM extraction stands; no hypothetical future hidden-field
   regression is asserted. Cost: changed markup must keep real CI passing, and
   this is not a general accessibility certification.
5. No special workaround for hypothetical browser-context close failure: actual
   owned cleanup and OIDC outer error suppression remain. Cost: an unobserved
   future close failure may replace the fixed diagnostic; no demonstrated leak.
6. Product authorization/kernel/schema/UI redesign remains unchanged and outside
   this additive validation slice, separately covered by existing native gates.
   Cost: no new topology, authorization identity or production capacity guarantee.

Deferred minors: none in this review. Earlier increments' documented limits remain.
