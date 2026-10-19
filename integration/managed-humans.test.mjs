// SPDX-License-Identifier: GPL-3.0-or-later
// In-memory helper orchestration regressions, NOT native auth/kernel evidence.
import test from 'node:test'
import assert from 'node:assert/strict'
import { cycleRequester } from './managed-humans.mjs'

const id = 'principal:account-' + '1'.repeat(32)
async function run({ generationAware = true, retarget = false, sameSession = false } = {}) {
  let disabled = false, selected = false, version = 1, serial = 0
  const sessions = new Map([['old', 1]])
  const locator = {
    fill: async () => {}, check: async () => {}, waitFor: async () => {},
    setChecked: async value => { selected = value },
    getByRole: (_role, opts) => ({ ...locator, click: async () => {
      if (opts.name === 'Save account') { disabled = selected; version++ }
    } }), getByText: () => locator, filter: () => locator,
  }
  const page = { ...locator, getByLabel: () => locator, locator: () => locator,
    waitForResponse: async () => ({ status: () => 200 }), waitForFunction: async () => {}, evaluate: async () => ({}) }
  const browser = { newContext: async () => makeContext('independent') }
  function makeContext(cookie) {
    const ctx = { cookie, cookies: async () => [{ name: 'picketx_session', value: ctx.cookie }],
      addCookies: async cookies => { ctx.cookie = cookies[0].value },
      browser: () => browser, newPage: async () => ({ context: ctx }), close: async () => {},
      request: { get: async () => {
        const status = sessions.has(ctx.cookie) && !disabled && (!generationAware || sessions.get(ctx.cookie) === version) ? 200 : 401
        if (status === 401) sessions.delete(ctx.cookie) // Core's unauthorized side effect
        return { status: () => status, json: async () => ({ id, roles: ['role:requester'] }) }
      } } }
    return ctx
  }
  const context = makeContext('old'), requester = { context }
  const grant = { uid: 'unit-grant', expires_at: '2026-10-19T09:00:00Z' }
  const report = { managedAccounts: { logins: [{ id }] }, humanRequests: [{ grantUID: 'unit-grant', grant }], confirmations: [{ revision: 4 }] }
  const output = console.log; console.log = () => {} // no native-looking PASS lines from unit doubles
  try { await cycleRequester({ page, requester, context, origin: 'http://unit.invalid', report,
    confirmed: async () => { report.confirmations.push({ revision: 4 }); return 4 },
    scenario: async () => {}, api: async () => ({ grants: [retarget ? { ...grant, expires_at: '2026-10-19T10:00:00Z' } : structuredClone(grant)] }),
    signIn: async person => { person.context.cookie = sameSession ? 'old' : 'fresh-' + ++serial; sessions.set(person.context.cookie, version) },
  }) } finally { console.log = output }
  return report.managedAccounts.lifecycle
}

test('helper rejects generation-blind resolver even though disabled401 destroys first session', async () => {
  await assert.rejects(() => run({ generationAware: false }), { code: 'ERR_ASSERTION' })
})
test('helper accepts independent untouched stale sessions with generation-aware resolution', async () => {
  const l = await run()
  assert.deepEqual(l.sessions, { disabledBefore: 200, revivalBefore: 200, distinct: true })
  assert.equal(l.disabledMe, 401); assert.equal(l.oldCookieMe, 401)
})
test('helper rejects Grant deadline extension without publication advancement', async () => {
  await assert.rejects(() => run({ retarget: true }), /account edit must preserve exact full Grant/)
})
test('helper rejects two contexts sharing the same server-side session', async () => {
  await assert.rejects(() => run({ sameSession: true }), { code: 'ERR_ASSERTION' })
})
