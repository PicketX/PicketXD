# Evidence case identity plan

Spec: docs/superpowers/specs/2026-10-19-evidence-case-identity.md
Context map reviewed: integration/console-evidence.mjs (case membership/uniqueness),
integration/console-evidence.test.mjs (unit mutation contract), console-kernel.mjs
(real string producer), validation.yml (complete native gates), M2D2 docs (boundaries).
Base: localaf4e1a1, remote mainfb763f7, same tree4ea57e1ba5fb250a9110e68570c8b30a67d5220c.
Inline execution under standing authority. No production or dependency changes.

## Task 1: enforce string case identity
1. Add array-name/same-length substitution regression. Run Node evidence suite.
   Expected: RED, a missing required case is incorrectly accepted.
2. Require typeof name string before own-property and uniqueness lookup.
3. Run evidence suite, syntax/diff checks and real archived-report validation.
   Expected: GREEN, all current real observations unchanged.
4. One independent whole-branch review; full Linux CI including original native and
   systemd gates; expected-head merge and main CI verification.

Review focus: string coercion, unique required-case coverage, real producer compatibility,
no weakened observation/status/expiry checks, source tree and native CI evidence.
