// SPDX-License-Identifier: GPL-3.0-or-later
// Native-only browser workflow; no route stubs or synthetic execution reports.
import assert from 'node:assert/strict'
import { join } from 'node:path'
import { validateHumanApproval } from './console-evidence.mjs'
import { cycleRequester } from './managed-humans.mjs'

export function assertDeadline(actual, expected) {
  const observed = Date.parse(actual), wanted = Date.parse(expected)
  assert(Number.isFinite(observed) && Number.isFinite(wanted), 'valid absolute deadlines required')
  assert.equal(observed, wanted, 'the absolute instant must not change')
}

export async function runHumanRequests({ browser, origin, host, password, evidence, report, api, confirmed, scenario, appHits, adminPage }) {
  const contexts = await Promise.all([browser.newContext(), browser.newContext()])
  const [requester, approver] = await Promise.all(contexts.map(c => c.newPage()))
  const errors = []
  try {
    for (const page of [requester, approver]) page.on('pageerror', e => errors.push(e.message))
    async function signIn(page, user) {
      await page.goto(origin + '/console/')
      await page.getByLabel('Sign-in method', { exact: true }).selectOption('password')
      await page.getByLabel('Username', { exact: true }).fill(user)
      await page.getByLabel('Password', { exact: true }).fill(password)
      const response = page.waitForResponse(r => r.url().endsWith('/login/password') && r.request().method() === 'POST')
      await page.getByRole('button', { name: 'Sign in', exact: true }).click()
      assert.equal((await response).status(), 200, `actual human ${user} login`)
      const me = await page.evaluate(async () => {
        const r = await fetch('/console/api/v1/me'); if (!r.ok) throw Error(`identity ${r.status}`); return r.json()
      })
      const account = report.managedAccounts.directory.accounts.find(a => a.username === user)
      assert.equal(me.id, account.id); assert.deepEqual(me.roles, account.roles)
      await page.getByRole('button', { name: 'Access requests', exact: true }).click()
      await page.getByRole('heading', { name: 'Access requests', exact: true }).waitFor()
      return me
    }
    for (const [page, user] of [[requester, 'requester'], [approver, 'approver']]) report.managedAccounts.logins.push(await signIn(page, user))
    const identities = Object.fromEntries(report.managedAccounts.directory.accounts.map(a => [a.username, a.id]))
    assert.equal(await requester.getByLabel('Policy YAML').count(), 0)
    assert.equal(await requester.evaluate(async () => (await fetch('/console/api/v1/publication', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: '{}' })).status), 403)
    const row = (page, reason) => page.locator('.request-row').filter({ hasText: reason })
    const read = async reason => requester.evaluate(async reason => {
      const response = await fetch('/console/api/v1/access-requests?limit=50')
      if (!response.ok) throw Error(`request history ${response.status}`)
      return (await response.json()).requests.find(r => r.reason === reason)
    }, reason)
    async function create(family, suffix = '') {
      await requester.getByRole('button', { name: 'Use latest request catalog', exact: true }).click()
      const reason = `Native human IPv${family}${suffix}`
      await requester.getByLabel('Requested source IP or CIDR', { exact: true }).fill(family === 4 ? '127.0.0.1' : '::1')
      await requester.getByLabel('Requested Resource', { exact: true }).selectOption('resource-demo-v1')
      for (const name of ['web TCP/18445', 'sibling TCP/18446']) await requester.getByRole('checkbox', { name, exact: true }).check()
      await requester.locator('#request-duration').fill('180')
      await requester.getByLabel('Request reason', { exact: true }).fill(reason)
      const response = requester.waitForResponse(r => r.url().endsWith('/access-requests') && r.request().method() === 'POST')
      await requester.getByRole('button', { name: 'Send access request', exact: true }).click()
      assert.equal((await response).status(), 201)
      await row(requester, reason).getByText('Pending', { exact: true }).waitFor()
      const original = await read(reason)
      assert.equal(original.requester, identities.requester)
      assert.equal(original.source, family === 4 ? '127.0.0.1/32' : '::1/128')
      return { family, reason, original }
    }
    async function approve(input, lifetime) {
      await approver.getByRole('button', { name: 'Use latest request catalog', exact: true }).click()
      await row(approver, input.reason).getByRole('button', { name: 'Review request', exact: true }).click()
      await approver.getByRole('checkbox', { name: 'Approve sibling', exact: true }).uncheck()
      const expiry = new Date(Date.now() + lifetime).toISOString()
      await approver.locator('#decision-expiry').fill(expiry)
      await approver.getByLabel('Decision reason', { exact: true }).fill(`Native narrowed IPv${input.family}`)
      await approver.getByLabel('Review confirmed', { exact: true }).check()
      const response = approver.waitForResponse(r => r.url().endsWith('/decision') && r.request().method() === 'POST')
      await approver.getByRole('button', { name: 'Approve and publish Grant', exact: true }).click()
      assert.equal((await response).status(), 200)
      await row(approver, input.reason).getByText('Approved', { exact: true }).waitFor()
      const accepted = await read(input.reason)
      assert.equal(accepted.uid, input.original.uid)
      assert.equal(accepted.requester, input.original.requester)
      assert.equal(accepted.source, input.original.source)
      assert.deepEqual(accepted.resource, input.original.resource)
      assert.equal(accepted.expires_at, input.original.expires_at, 'original deadline immutable')
      assertDeadline(accepted.approved_expires_at, expiry)
      assert.equal(accepted.approver, identities.approver)
      assert.deepEqual(accepted.approved_endpoints, ['web'])
      assert.equal(await confirmed(), accepted.revision, 'exact approved revision applied')
      const grant = (await api('grants?limit=100')).grants.find(g => g.uid === accepted.grant_uid)
      assert(grant, 'approved Grant persisted')
      assert.equal(grant.source, accepted.source)
      assertDeadline(grant.expires_at, expiry)
      assert.deepEqual(grant.scopes[0].endpoint_names, ['web'])
      const audit = (await api('audit?limit=100')).events.find(e => e.revision === accepted.revision)
      assert(audit, 'independent human audit persisted')
      const proof = { family: input.family, uid: accepted.uid, requester: accepted.requester, approver: accepted.approver,
        source: accepted.source, requestedEndpoints: accepted.endpoint_names, approvedEndpoints: accepted.approved_endpoints,
        requestedExpiry: accepted.expires_at, approvedExpiry: accepted.approved_expires_at, grantUID: grant.uid, grantSource: grant.source,
        grantEndpoints: grant.scopes[0].endpoint_names, grantExpiry: grant.expires_at, revision: accepted.revision,
        resource: { name: accepted.resource.name, uid: accepted.resource.uid }, decisionReason: accepted.decision_reason,
        grant, auditActor: audit.actor, auditReason: audit.reason, auditRevision: audit.revision }
      validateHumanApproval(proof, identities)
      report.humanRequests.push(proof)
      return accepted
    }
    report.humanRequests = []
    const baseRevision = (await api('publication')).revision
    report.humanTimeline = { baseline: baseRevision }
    const four = await create(4)
    assert.equal((await api('publication')).revision, baseRevision, 'pending request does not publish')
    await scenario('humanPending4')
    const approved4 = await approve(four, 90000)
    report.humanTimeline.approved4 = approved4.revision
    await scenario('humanApproved4', { persistent: true })
    const six = await create(6)
    assert.equal((await api('publication')).revision, approved4.revision)
    await scenario('humanPending6')
    const approved6 = await approve(six, 35000)
    report.humanTimeline.approved6 = approved6.revision
    await scenario('humanApproved6', { persistent: true })
    await scenario('humanSibling', { port: 18446 })
    const app = await contexts[0].newPage(), before = appHits()
    const reached = await app.goto(`http://${host}/allowed`, { timeout: 3000 })
    assert.equal(reached.status(), 200)
    assert.equal(await app.locator('body').textContent(), 'protected real HTTP application')
    assert.equal(appHits() - before, 1)
    report.cases.push({ name: 'humanBrowser', revision: report.confirmations.at(-1).revision, status: 'pass', observations: [{ family: 4, outcome: 'allow', status: 200, appDelta: appHits() - before }] })
    await app.close()
    await cycleRequester({ page: adminPage, requester, context: contexts[0], origin, report, confirmed, scenario, signIn })
    for (const width of [375, 1440]) {
      await requester.setViewportSize({ width, height: 1000 })
      await requester.getByRole('button', { name: 'Refresh requests', exact: true }).click()
      await row(requester, six.reason).getByText('Approved', { exact: true }).waitFor()
      assert(await requester.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await requester.screenshot({ path: join(evidence, `human-requests-${width}.png`), fullPage: true })
    }
    await approver.getByRole('button', { name: 'Grants', exact: true }).click()
    await approver.getByRole('button', { name: 'Use latest Grant revision', exact: true }).click()
    await approver.locator(`[data-grant-id="${approved4.grant_uid}"]`).getByRole('button', { name: 'Revoke Grant', exact: true }).click()
    await approver.getByLabel('Revocation reason', { exact: true }).fill('Native human approved IPv4 revoke')
    const revoked = approver.waitForResponse(r => r.url().endsWith(`/grants/${approved4.grant_uid}/revoke`))
    await approver.getByRole('button', { name: 'Confirm revocation', exact: true }).click()
    assert.equal((await revoked).status(), 200); report.humanTimeline.revoked4 = await confirmed()
    await scenario('humanRevoke4'); await scenario('humanEstablishedRevoke4', { persistent: true })
    const revision = (await api('publication')).revision
    await new Promise(r => setTimeout(r, Math.max(0, Date.parse(approved6.approved_expires_at) + 250 - Date.now())))
    await scenario('humanExpired'); await scenario('humanEstablishedExpired6', { persistent: true })
    report.humanTimeline.expired = (await api('publication')).revision
    assert.equal(report.humanTimeline.expired, revision, 'expiry did not republish or renew')
    await requester.getByRole('button', { name: 'Refresh requests', exact: true }).click()
    await row(requester, four.reason).getByText('Desired Grant: revoked', { exact: true }).waitFor()
    await row(requester, six.reason).getByText('Desired Grant: expired', { exact: true }).waitFor()
    for (const [i, input] of [four, six].entries()) report.humanRequests[i].finalGrantState = (await read(input.reason)).grant_state
    const rejected = await create(6, ' rejected')
    await approver.getByRole('button', { name: 'Access requests', exact: true }).click()
    await approver.getByRole('button', { name: 'Use latest request catalog', exact: true }).click()
    await row(approver, rejected.reason).getByRole('button', { name: 'Review request', exact: true }).click()
    await approver.getByLabel('Decision reason', { exact: true }).fill('Native independent rejection')
    const rejectedResponse = approver.waitForResponse(r => r.url().endsWith('/decision'))
    await approver.getByRole('button', { name: 'Reject request', exact: true }).click()
    assert.equal((await rejectedResponse).status(), 200)
    await row(approver, rejected.reason).getByText('Rejected', { exact: true }).waitFor()
    report.humanTimeline.rejected = (await api('publication')).revision
    assert.equal(report.humanTimeline.rejected, revision, 'rejection does not publish')
    assert.equal((await read(rejected.reason)).grant_uid, undefined)
    await scenario('humanRejected')
    assert.deepEqual(errors, [])
    console.log('PASS actual human sessions/request/narrowed approval → confirmed Agent dual-stack L4/L7 → revoke/absolute expiry/rejection')
  } finally { await Promise.all(contexts.map(c => c.close())) }
}
