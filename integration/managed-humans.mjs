// SPDX-License-Identifier: GPL-3.0-or-later
// Real console operations and fail-closed public evidence; never stores secrets.
import assert from 'node:assert/strict'
import { join } from 'node:path'

export function validateManagedHumans(report) {
  const { directory, logins } = report.managedAccounts
  assert.equal(directory.mode, 'database')
  assert.equal(directory.revision, 2)
  assert.equal(directory.accounts.length, 2)
  assert.equal(directory.events.length, 2)
  assert.equal(logins.length, 2)
  const identities = {}
  for (const [i, [username, role]] of [['requester', 'role:requester'], ['approver', 'role:admin']].entries()) {
    const account = directory.accounts.find(a => a.username === username)
    assert(account, 'actual provisioned account required')
    assert.match(account.id, /^principal:account-[a-f0-9]{32}$/)
    assert.equal(account.disabled, false); assert.equal(account.version, 1)
    assert.deepEqual(account.roles, [role])
    assert.equal(logins[i].id, account.id)
    assert.deepEqual(logins[i].roles, [role])
    assert(!Object.values(identities).includes(account.id), 'independent human identities')
    identities[username] = account.id
    assert.deepEqual(directory.events[i === 0 ? 1 : 0], {
      ...directory.events[i === 0 ? 1 : 0], actor: 'principal:admin', accountId: account.id,
      operation: 'create', reason: `Native onboard ${username}`, revision: i + 1,
    })
  }
  const l = report.managedAccounts.lifecycle
  assert.deepEqual(l.sessions, { disabledBefore: 200, revivalBefore: 200, distinct: true })
  assert.equal(l.disabledMe, 401); assert.equal(l.oldCookieMe, 401)
  assert.equal(l.freshMe.id, identities.requester); assert.deepEqual(l.freshMe.roles, ['role:requester'])
  assert.equal(l.publicationBefore, report.humanTimeline.approved6)
  assert.equal(l.afterDisable, l.publicationBefore); assert.equal(l.afterEnable, l.publicationBefore)
  for (const grants of [l.disabledGrants, l.enabledGrants]) assert.deepEqual(grants, report.humanRequests.map(r => r.grant), 'account lifecycle preserves both complete approved Grants and absolute deadlines')
  const original = directory.accounts.find(a => a.id === identities.requester)
  let previous = directory
  for (const [state, disabled, version, revision, reason] of [
    [l.disabledDirectory, true, 2, 3, 'Native disable requester'],
    [l.enabledDirectory, false, 3, 4, 'Native re-enable requester'],
  ]) {
    assert.equal(state.mode, 'database'); assert.equal(state.revision, revision)
    assert.deepEqual(state.accounts, directory.accounts.map(a => a.id === original.id ? { ...a, disabled, version } : a))
    assert.equal(state.events.length, revision)
    assert.deepEqual(state.events.slice(1), previous.events, 'all previous account audit retained')
    const event = state.events[0]
    assert.equal(event.actor, 'principal:admin'); assert.equal(event.accountId, original.id)
    assert.equal(event.operation, 'update'); assert.equal(event.reason, reason); assert.equal(event.revision, revision)
    previous = state
  }
  return identities
}

export async function provisionManagedHumans({ page, password, evidence, report }) {
  await page.getByRole('button', { name: 'Accounts', exact: true }).click()
  await page.getByText('No human accounts yet.', { exact: true }).waitFor()
  for (const [username, role] of [['requester', 'requester'], ['approver', 'admin']]) {
    await page.getByLabel('Account username', { exact: true }).fill(username)
    await page.getByLabel('Account password', { exact: true }).fill(password)
    await page.getByRole('checkbox', { name: 'Requester', exact: true }).setChecked(role === 'requester')
    await page.getByRole('checkbox', { name: 'Admin', exact: true }).setChecked(role === 'admin')
    await page.getByLabel('Account change reason', { exact: true }).fill(`Native onboard ${username}`)
    const response = page.waitForResponse(r => r.url().endsWith('/accounts') && r.request().method() === 'POST')
    await page.getByRole('button', { name: 'Create account', exact: true }).click()
    assert.equal((await response).status(), 201)
    await page.locator('.account-row').getByRole('heading', { name: username, exact: true }).waitFor()
    await page.waitForFunction(() => document.querySelector('.accounts-section')?.getAttribute('aria-busy') === 'false')
    assert.equal(await page.getByLabel('Account password', { exact: true }).inputValue(), '')
  }
  const directory = await page.evaluate(async () => {
    const r = await fetch('/console/api/v1/accounts'); if (!r.ok) throw Error(`directory ${r.status}`); return r.json()
  })
  assert(!JSON.stringify(directory).includes(password)); assert(!JSON.stringify(directory).includes('$2'))
  report.managedAccounts = { directory, logins: [] }
  for (const width of [375, 1440]) {
    await page.setViewportSize({ width, height: 1000 })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.screenshot({ path: join(evidence, `managed-accounts-${width}.png`), fullPage: true })
  }
  console.log('PASS actual Accounts provisioning: database authority, durable IDs and audited roles')
}

export async function cycleRequester({ page, requester, context, origin, report, confirmed, scenario, signIn, api }) {
  const untouched = await context.browser().newContext()
  try {
  // A rejected /me destroys its server-side SCS session. A second, independently
  // logged-in session must remain completely unprobed while the account is disabled.
  await signIn(await untouched.newPage(), 'requester')
  const disabledBefore = (await context.request.get(origin + '/console/api/v1/me')).status()
  const revivalBefore = (await untouched.request.get(origin + '/console/api/v1/me')).status()
  assert.equal(disabledBefore, 200); assert.equal(revivalBefore, 200)
  const session = cookies => cookies.find(c => c.name === 'picketx_session')?.value
  const disabledCookie = session(await context.cookies()), revivalCookie = session(await untouched.cookies())
  assert(disabledCookie && revivalCookie); assert.notEqual(disabledCookie, revivalCookie)
  await page.getByRole('button', { name: 'Accounts', exact: true }).click()
  await page.getByRole('heading', { name: 'requester', exact: true }).waitFor()
  const row = page.locator('.account-row').filter({ has: page.getByRole('heading', { name: 'requester', exact: true }) })
  const lifecycle = report.managedAccounts.lifecycle = { publicationBefore: report.confirmations.at(-1).revision,
    sessions: { disabledBefore, revivalBefore, distinct: true } }
  for (const disabled of [true, false]) {
    await row.getByRole('button', { name: 'Edit account', exact: true }).click()
    await page.getByLabel('Account disabled', { exact: true }).setChecked(disabled)
    await page.getByLabel('Account change reason', { exact: true }).fill(disabled ? 'Native disable requester' : 'Native re-enable requester')
    await page.getByLabel('Invalidate previous sessions', { exact: true }).check()
    const response = page.waitForResponse(r => r.url().includes('/accounts/') && r.request().method() === 'PUT')
    await page.getByRole('button', { name: 'Save account', exact: true }).click()
    assert.equal((await response).status(), 200)
    await row.getByText(disabled ? 'Disabled' : 'Enabled', { exact: true }).waitFor()
    await page.waitForFunction(() => document.querySelector('.accounts-section')?.getAttribute('aria-busy') === 'false')
    lifecycle[disabled ? 'disabledDirectory' : 'enabledDirectory'] = await page.evaluate(async () => {
      const r = await fetch('/console/api/v1/accounts'); if (!r.ok) throw Error(`directory ${r.status}`); return r.json()
    })
    // Never touch the revival session while disabled: restoring a cookie for a
    // previously destroyed session would falsely pass a generation-blind resolver.
    lifecycle[disabled ? 'disabledMe' : 'oldCookieMe'] = (await (disabled ? context : untouched).request.get(origin + '/console/api/v1/me')).status()
    assert.equal(lifecycle[disabled ? 'disabledMe' : 'oldCookieMe'], 401)
    const grants = (await api('grants?limit=100')).grants
    lifecycle[disabled ? 'disabledGrants' : 'enabledGrants'] = report.humanRequests.map(r => {
      const actual = grants.find(g => g.uid === r.grantUID)
      assert.deepEqual(actual, r.grant, 'account edit must preserve exact full Grant, including original absolute expiry')
      return actual
    })
    lifecycle[disabled ? 'afterDisable' : 'afterEnable'] = await confirmed()
    assert.equal(report.confirmations.at(-1).revision, lifecycle.publicationBefore, 'account edits never republish Grants')
    await scenario(disabled ? 'humanAccountDisabled' : 'humanAccountReenabled')
  }
  await signIn(requester, 'requester')
  const me = await context.request.get(origin + '/console/api/v1/me')
  assert.equal(me.status(), 200); lifecycle.freshMe = await me.json()
  assert.equal(lifecycle.freshMe.id, report.managedAccounts.logins[0].id)
  console.log('PASS actual account disable/re-enable: permanent old-cookie invalidation, independent dual-stack Grants unchanged')
  } finally { await untouched.close() }
}
