// SPDX-License-Identifier: GPL-3.0-or-later
import assert from 'node:assert/strict'
import { join } from 'node:path'

const fields = ['id', 'caller', 'route', 'source', 'allowed', 'reason', 'revision', 'grant_ids', 'deny_ids']
const integer = (n, min = 1) => Number.isSafeInteger(n) && n >= min
const keys = (value, names) => value !== null && typeof value === 'object' && !Array.isArray(value) &&
  Object.getPrototypeOf(value) === Object.prototype && Object.keys(value).sort().join(',') === [...names].sort().join(',')
const project = row => Object.fromEntries(fields.map(key => [key, row[key]]))
const invalid = () => new Error('Invalid installed HTTP audit evidence')

export function validateHTTPAuditEvidence(receipt, report) {
  // Do not serialize rejected input into public failures (including assertion diffs).
  try {
    assert(keys(receipt, ['mode', 'revision', 'principal', 'grants', 'probes', 'rows', 'viewports']))
    assert.equal(receipt.mode, report.oidc ? 'oidc' : 'password')
    assert(integer(receipt.revision)); assert.equal(receipt.revision, report.deployment.restart.revision)
    assert(report.confirmations.some(c => c.revision === receipt.revision && c.generation === receipt.revision &&
      c.appliedGeneration === receipt.revision && c.enforcementConfirmed === true))
    const account = report.managedAccounts.directory.accounts.find(a => a.username === 'approver')
    assert(account && /^principal:account-[a-f0-9]{32}$/.test(account.id))
    assert.equal(receipt.principal, account.id)
    assert(Array.isArray(receipt.grants) && receipt.grants.length === 2 && new Set(receipt.grants).size === 2)
    assert(receipt.grants.every(id => typeof id === 'string' && id.length > 0 && id.length <= 256))
    assert(Array.isArray(report.grantIssuances) && report.grantIssuances.length === 2)
    report.grantIssuances.forEach((issued, i) => {
      assert(keys(issued, ['family', 'source', 'uid', 'revision']))
      assert.equal(issued.family, [4, 6][i]); assert.equal(issued.source, ['127.0.0.1', '::1'][i])
      assert.equal(issued.uid, receipt.grants[i]); assert(integer(issued.revision))
      assert.equal(issued.revision, receipt.revision - 1 + i)
      const cases = report.cases.filter(c => c.name === ['grant4', 'grant6'][i])
      assert.equal(cases.length, 1); assert.equal(cases[0].revision, issued.revision)
    })
    assert(Array.isArray(receipt.probes) && receipt.probes.length === 4)
    assert(Array.isArray(receipt.rows) && receipt.rows.length === 4)
    assert.deepEqual(receipt.viewports, [1440, 375])
    let cursor = 0
    receipt.probes.forEach((probe, i) => {
      assert(keys(probe, ['name', 'family', 'before', 'event', 'status', 'appDelta']))
      assert.equal(probe.name, ['allow4', 'deny4', 'allow6', 'deny6'][i])
      const family = i < 2 ? 4 : 6, allowed = i % 2 === 0
      assert.equal(probe.family, family)
      assert(integer(probe.before, 0) && probe.before >= cursor)
      const e = probe.event
      assert(keys(e, fields)); assert(integer(e.id) && e.id > probe.before)
      assert.equal(e.caller, 'initial-ingress'); assert.equal(e.route, 'route-v1')
      assert.equal(e.source, family === 4 ? '127.0.0.1' : '::1')
      assert.equal(e.allowed, allowed); assert.equal(e.reason, allowed ? 'allowed' : 'route-mismatch')
      assert.equal(e.revision, receipt.revision)
      assert.deepEqual(e.grant_ids, allowed ? [receipt.grants[family === 4 ? 0 : 1]] : [])
      assert.deepEqual(e.deny_ids, [])
      assert.equal(probe.status, allowed ? 200 : 403); assert.equal(probe.appDelta, allowed ? 1 : 0)
      assert(keys(receipt.rows[i], fields)); assert.deepEqual(receipt.rows[i], e)
      cursor = e.id
    })
  } catch { throw invalid() }
}

export async function runInstalledHTTPAudit({ browser, origin, password, externalSignIn, report, api, request, evidence, grants }) {
  let context
  try {
    const receipt = { mode: externalSignIn ? 'oidc' : 'password', revision: report.deployment.restart.revision,
      principal: '', grants, probes: [], rows: [], viewports: [1440, 375] }
    // Drain the actual ascending keyset, never guess the latest event from a timestamp.
    async function read(after) {
      const page = await api(`http-decisions?after=${after}&limit=100`)
      assert(Array.isArray(page.events) && integer(page.next_after, 0))
      let cursor = after
      for (const event of page.events) { assert(integer(event.id) && event.id > cursor); cursor = event.id }
      assert.equal(page.next_after, cursor)
      return page
    }
    async function tail() {
      let after = 0
      for (let i = 0; i < 20; i++) {
        const page = await read(after); after = page.next_after
        if (page.events.length === 0) return after
      }
      throw invalid()
    }
    for (const [i, name] of ['allow4', 'deny4', 'allow6', 'deny6'].entries()) {
      const family = i < 2 ? 4 : 6, before = await tail()
      const result = await request(family, { path: i % 2 === 0 ? '/allowed' : '/denied' })
      const page = await read(before)
      assert.equal(page.events.length, 1, 'exactly one actual decision for this serial request')
      receipt.probes.push({ name, family, before, event: project(page.events[0]), status: result.status, appDelta: result.appDelta })
    }
    context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
    const page = await context.newPage(), errors = []
    page.on('pageerror', () => errors.push('pageerror'))
    if (externalSignIn) await externalSignIn(page, 'approver', report)
    else {
      await page.goto(origin + '/console/')
      await page.getByLabel('Sign-in method', { exact: true }).selectOption('password')
      await page.getByLabel('Username', { exact: true }).fill('approver')
      await page.getByLabel('Password', { exact: true }).fill(password)
      const response = page.waitForResponse(r => r.url().endsWith('/login/password') && r.request().method() === 'POST')
      await page.getByRole('button', { name: 'Sign in', exact: true }).click()
      assert.equal((await response).status(), 200)
    }
    const me = await page.evaluate(async () => {
      const r = await fetch('/console/api/v1/me'); if (!r.ok) throw Error('identity unavailable'); return r.json()
    })
    const account = report.managedAccounts.directory.accounts.find(a => a.username === 'approver')
    assert.equal(me.id, account.id); assert.deepEqual(me.roles, account.roles)
    assert(me.operations.includes('audit:read')); receipt.principal = me.id
    await page.getByRole('button', { name: 'Audit', exact: true }).click()
    await page.getByRole('button', { name: 'HTTP decisions', exact: true }).click()
    const section = page.locator('.http-audit-section')
    await page.locator('.http-audit-section[aria-busy="false"]').waitFor()
    for (const { event } of receipt.probes) {
      const article = page.locator(`[data-http-decision-id="${event.id}"]`)
      for (let i = 0; await article.count() === 0 && i < 20; i++) {
        const after = await section.getAttribute('data-after')
        const response = page.waitForResponse(r => r.url().includes('/http-decisions?') && r.request().method() === 'GET')
        await page.getByRole('button', { name: 'Read next decisions', exact: true }).click()
        assert.equal((await response).status(), 200)
        await page.locator('.http-audit-section[aria-busy="false"]').waitFor()
        assert.notEqual(await section.getAttribute('data-after'), after, 'event must exist in actual keyset')
      }
      await article.waitFor()
      await article.getByText('Decision details', { exact: true }).click()
      await article.locator('.http-contributions').first().waitFor()
      receipt.rows.push(await article.evaluate(node => {
        const d = [...node.querySelectorAll('dd')].map(e => e.textContent)
        const lists = [...node.querySelectorAll('.http-contributions')]
        return { id: Number(d[4]), caller: d[3], route: d[2], source: d[0], allowed: node.querySelector('h3').textContent === 'Allowed',
          reason: d[1], revision: Number(d[5]), grant_ids: [...lists[0].querySelectorAll('li')].map(e => e.textContent),
          deny_ids: [...lists[1].querySelectorAll('li')].map(e => e.textContent) }
      }))
    }
    validateHTTPAuditEvidence(receipt, report)
    for (const width of receipt.viewports) {
      await page.setViewportSize({ width, height: 1000 })
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'audit must fit viewport')
      assert(await page.evaluate(() => localStorage.length === 0 && sessionStorage.length === 0), 'no browser credential storage')
      await page.screenshot({ path: join(evidence, `http-audit-${width}.png`), fullPage: true })
    }
    assert.equal(errors.length, 0)
    return receipt
  } catch { throw invalid() }
  finally { await context?.close() }
}
