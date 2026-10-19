// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'

export function assertServicesAlive(children) {
  for (const child of children) {
    assert.equal(child.exitCode, null, 'required subprocess remains alive')
    assert.equal(child.signalCode, null, 'required subprocess must not have terminated by signal')
  }
}
