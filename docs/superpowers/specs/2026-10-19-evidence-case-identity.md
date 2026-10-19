# Evidence case identity follow-up

The native report contains correct string names, but a mutation of grant4 to an
array ["baseline"] with baseline observations is accepted by the validator after
the own-property fix. Property lookup coerces the array to a string while Set
uniqueness compares the array object itself. A required case can therefore disappear.

Require names to be strings before membership/uniqueness checks. Exact case count,
own membership and string uniqueness together require every case exactly once.
Preserve all current native cases, expected observations and source confirmations.
No production change. RED→GREEN regressions must replace a real required case with
non-string names without changing report length; original actual report still passes.

中文：补齐证据名称的类型校验，拒绝数组等非字符串隐式转换造成的必需场景缺失。
实际原生报告正常，不撤回已观察到的流量结果；修复后仍需完整 Linux CI 和独立评审。
