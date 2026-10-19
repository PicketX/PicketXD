// SPDX-License-Identifier: GPL-3.0-or-later
// Native fixture only. No application/password/SSH proxy in product code.
import assert from 'node:assert/strict'
import { readFile, writeFile, mkdir, chmod, chown } from 'node:fs/promises'
import { join } from 'node:path'
import { homedir } from 'node:os'
import { execFileSync, execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { createHash, randomBytes } from 'node:crypto'
import https from 'node:https'
import http from 'node:http'
import { assertServicesAlive } from './process-health.mjs'

export const giteaSHA256 = '15681fc615d56611c274f52803fef4d6d5316b620f93b677dd4b831a74e50b9f'
const caseNames = ['baseline', 'httpsOnly', 'both', 'revokeHTTPS4', 'revokeSSH4', 'revokeHTTPS6', 'revokeSSH6']
const address = family => family === 4 ? '127.0.0.1' : '::1'
const authority = 'gitea.example:19445'
const command = promisify(execFile)

export function validateGiteaEvidence(p, confirmations) {
  assert.equal(p.version, '28.2.0'); assert.equal(p.binarySHA256, giteaSHA256)
  assert.match(p.oid, /^[a-f0-9]{40}$/)
  assert.equal(p.cases.length, 7); assert.equal(p.grants.length, 4)
  const baseline = p.cases[0].revision
  const confirmed = revision => { assert(Number.isSafeInteger(revision) && revision > 0); assert(confirmations.some(c => c.revision === revision), 'actual Gitea revision confirmation required') }
  const ids = new Set()
  for (const [i, g] of p.grants.entries()) {
    const family = i < 2 ? 4 : 6, endpoint = i % 2 === 0 ? 'https' : 'ssh'
    assert.equal(g.family, family); assert.equal(g.endpoint, endpoint)
    assert.equal(g.revision, baseline + 1 + (family === 4 ? 0 : 1) + (endpoint === 'ssh' ? 2 : 0)); confirmed(g.revision)
    const v = g.grant
    assert.equal(typeof v.uid, 'string'); assert(v.uid.length > 0 && !ids.has(v.uid)); ids.add(v.uid)
    assert.equal(v.name, `gitea-${endpoint}-${family}`)
    assert.equal(v.source, family === 4 ? '127.0.0.1/32' : '::1/128')
    assert.deepEqual(v.scopes, [{ resource_name: 'gitea', resource_uid: 'resource-gitea-v1', selection: 'named', endpoint_names: [endpoint] }])
    assert.equal(v.mode, 'bounded'); assert.equal(v.revoked, false); assert(Number.isFinite(Date.parse(v.expires_at)))
  }
  p.cases.forEach((c, index) => {
    assert.equal(c.name, caseNames[index]); assert.equal(c.revision, baseline + [0, 2, 4, 5, 6, 7, 8][index]); confirmed(c.revision)
    assert.equal(c.observations.length, 4)
    c.observations.forEach((o, i) => {
      const family = i < 2 ? 4 : 6, endpoint = i % 2 === 0 ? 'https' : 'ssh'
      assert.equal(o.family, family); assert.equal(o.endpoint, endpoint)
      const denied = index === 0 || (index === 1 && endpoint === 'ssh') ||
        (index >= 3 && family === 4 && endpoint === 'https') || (index >= 4 && family === 4) ||
        (index >= 5 && endpoint === 'https') || index === 6
      assert.equal(o.outcome, denied ? 'transport-deny' : 'allow')
    })
  })
  assert.deepEqual(p.browser, { login: true, repository: true })
  assert.equal(p.applicationAuth.length, 2)
  p.applicationAuth.forEach((a, i) => assert.deepEqual(a, { family: [4, 6][i], anonymousHTTPS: 'denied', outsiderHTTPS: 'denied', outsiderSSH: 'denied', ownerHTTPS: 'allow', ownerSSH: 'allow', clonedOID: p.oid }))
  assert.deepEqual(p.establishedHTTPS4, { revision: baseline + 5, sameSocket: true, before: 200, after: 403, upstreamAfter: 0 })
  assert.deepEqual(p.tls, { untrustedRejected: true }); assert.deepEqual(p.ssh, { wrongHostKeyRejected: true })
  assert.equal(p.healthyControls, true)
}

export async function runGiteaApplication({ root, core, temp, evidence, chromium, page, publish, confirmed, api, start, children, ingressToken, secrets }) {
  const dir = join(temp, 'gitea'), binary = join(root, 'artifacts/gitea')
  const proof = { version: '28.2.0', binarySHA256: createHash('sha256').update(await readFile(binary)).digest('hex'), grants: [], cases: [], applicationAuth: [] }
  assert.equal(proof.binarySHA256, giteaSHA256)
  assert.match(execFileSync(binary, ['--version'], { encoding: 'utf8' }), /Gitea version 28\.2\.0/)
  await mkdir(dir, { mode: 0o750 }); await chmod(temp, 0o711)
  const pass = randomBytes(32).toString('hex'); secrets.push(pass)
  const run = (bin, args, opts = {}) => execFileSync(bin, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], ...opts })
  const file = async (name, content, mode = 0o600) => { const path = join(dir, name); await writeFile(path, content, { mode }); return path }
  const keys = {}
  for (const name of ['host', 'owner', 'outsider', 'wrong-host']) {
    keys[name] = join(dir, name)
    run('ssh-keygen', ['-q', '-t', 'ed25519', '-N', '', '-f', keys[name]])
    secrets.push((await readFile(keys[name], 'utf8')).trim())
  }
  const hostKey = (await readFile(keys.host + '.pub', 'utf8')).trim().split(' ').slice(0, 2).join(' ')
  const wrongKey = (await readFile(keys['wrong-host'] + '.pub', 'utf8')).trim().split(' ').slice(0, 2).join(' ')
  const known = await file('known_hosts', `[127.0.0.1]:19222 ${hostKey}\n[::1]:19222 ${hostKey}\n`)
  const wrongKnown = await file('wrong_known_hosts', `[127.0.0.1]:19222 ${wrongKey}\n`)
  run('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=PicketX native fixture CA', '-addext', 'basicConstraints=critical,CA:TRUE', '-keyout', join(dir, 'ca.key'), '-out', join(dir, 'ca.pem')])
  run('openssl', ['req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=gitea.example', '-keyout', join(dir, 'tls.key'), '-out', join(dir, 'tls.csr')])
  const ext = await file('tls.ext', 'subjectAltName=DNS:gitea.example,IP:127.0.0.1,IP:::1\nextendedKeyUsage=serverAuth\nbasicConstraints=CA:FALSE\n')
  run('openssl', ['x509', '-req', '-in', join(dir, 'tls.csr'), '-CA', join(dir, 'ca.pem'), '-CAkey', join(dir, 'ca.key'), '-CAcreateserial', '-days', '1', '-extfile', ext, '-out', join(dir, 'tls.pem')])
  for (const key of ['ca.key', 'tls.key']) { await chmod(join(dir, key), 0o600); secrets.push((await readFile(join(dir, key), 'utf8')).trim()) }
  const ca = await readFile(join(dir, 'ca.pem'))
  const ini = await file('app.ini', `APP_NAME = PicketX native Gitea\nRUN_USER = nobody\nRUN_MODE = prod\n[database]\nDB_TYPE = sqlite3\nPATH = ${dir}/gitea.db\n[repository]\nROOT = ${dir}/repositories\n[server]\nAPP_DATA_PATH = ${dir}/data\nDOMAIN = gitea.example\nROOT_URL = https://${authority}/\nHTTP_ADDR = 127.0.0.1\nHTTP_PORT = 19443\nSTART_SSH_SERVER = true\nSSH_DOMAIN = gitea.example\nSSH_USER = nobody\nSSH_LISTEN_HOST = ::\nSSH_LISTEN_PORT = 19222\nSSH_PORT = 19222\nSSH_SERVER_HOST_KEYS = ${keys.host}\n[security]\nINSTALL_LOCK = true\n[service]\nDISABLE_REGISTRATION = true\n[log]\nMODE = console\nLEVEL = Warn\n`)
  await chown(dir, 65534, 65534); await chown(ini, 65534, 65534)
  await chown(keys.host, 65534, 65534); await chown(keys.host + '.pub', 65534, 65534)
  const giteaArgs = ['--work-path', dir, '--config', ini]
  const giteaOptions = { uid: 65534, gid: 65534, cwd: dir }
  // Running the actual admin CLI migrates the private SQLite database first.
  run(binary, [...giteaArgs, 'migrate'], giteaOptions)
  for (const user of ['owner', 'outsider']) run(binary, [...giteaArgs, 'admin', 'user', 'create', '--username', user, '--password', pass, '--email', `${user}@example.invalid`, '--must-change-password=false', ...(user === 'owner' ? ['--admin'] : [])], giteaOptions)
  start('gitea', binary, [...giteaArgs, 'web'], giteaOptions)
  const until = async (check, label) => { for (let i = 0; i < 150; i++) { if (await check()) return; await new Promise(r => setTimeout(r, 100)) } throw Error(`Gitea timeout: ${label}`) }
  await until(async () => { try { return (await fetch('http://127.0.0.1:19443/api/v1/version')).ok } catch { return false } }, 'app startup')
  const appAPI = async (path, body, user = 'owner') => {
    const r = await fetch('http://127.0.0.1:19443/api/v1/' + path, { method: body ? 'POST' : 'GET', headers: { Authorization: 'Basic ' + Buffer.from(`${user}:${pass}`).toString('base64'), 'Content-Type': 'application/json' }, ...(body ? { body: JSON.stringify(body) } : {}) })
    assert(r.ok, `Gitea setup API ${path}: ${r.status}`); return r.json()
  }
  await appAPI('user/repos', { name: 'private', private: true, auto_init: true, default_branch: 'main', readme: 'Default' })
  for (const user of ['owner', 'outsider']) await appAPI('user/keys', { title: 'native', key: (await readFile(keys[user] + '.pub', 'utf8')).trim() }, user)
  const token = (await appAPI('users/owner/tokens', { name: 'native-git-read', scopes: ['read:repository'] })).sha1
  assert.equal(typeof token, 'string'); secrets.push(token)
  proof.oid = (await appAPI('repos/owner/private/branches/main')).commit.id
  assert.match(proof.oid, /^[a-f0-9]{40}$/)
  const askpass = await file('askpass.sh', '#!/bin/sh\ncase "$1" in *Username*) printf "%s\\n" "$GITEA_GIT_USER";; *) printf "%s\\n" "$GITEA_GIT_TOKEN";; esac\n', 0o700)
  let nginx = await readFile(join(core, 'examples/http/nginx.conf.template'), 'utf8')
  for (const [key, value] of Object.entries({ PREFIX: dir, PORT: 19445, APP_PORT: 19443, AUTHZ_PORT: 19091, TOKEN: ingressToken })) nginx = nginx.replaceAll(`__${key}__`, String(value))
  nginx = nginx.replace('listen 127.0.0.1:19445;', `listen 127.0.0.1:19445 ssl;\n        listen [::1]:19445 ssl ipv6only=on;\n        ssl_certificate ${dir}/tls.pem;\n        ssl_certificate_key ${dir}/tls.key;`)
    .replace('keepalive_timeout 30s;', 'keepalive_timeout 300s;')
    .replace('route-v1;', 'route-gitea-v1;').replace('proxy_set_header Host application.internal;', `proxy_set_header Host ${authority};\n            proxy_set_header X-Forwarded-Proto https;`)
    .replace('access_log off;', `log_format upstream '$upstream_addr';\n    access_log ${dir}/upstreams.log upstream;`)
    .replace('http {', 'http {\n    server { listen 127.0.0.1:19447; listen [::1]:19447 ipv6only=on; location / { return 200 "healthy"; } }')
  const conf = await file('nginx.conf', nginx)
  run('nginx', ['-t', '-p', dir + '/', '-c', conf])
  start('gitea-nginx', 'nginx', ['-p', dir + '/', '-c', conf, '-g', 'daemon off;'])
  const upstreamHits = async () => { try { return (await readFile(join(dir, 'upstreams.log'), 'utf8')).split('\n').filter(x => x.includes('127.0.0.1:19443')).length } catch { return 0 } }
  async function tlsRequest(family, { agent = false, trusted = true } = {}) {
    return new Promise((resolve, reject) => {
      let socket
      const req = https.get({ host: address(family), port: 19445, path: '/api/v1/version', servername: 'gitea.example', headers: { Host: authority }, ca: trusted ? ca : undefined, agent, timeout: 2500 }, res => {
        res.resume(); res.on('end', () => resolve({ status: res.statusCode, socket }))
      })
      req.on('socket', s => { socket = s }); req.on('error', reject); req.on('timeout', () => req.destroy(Error('HTTPS timeout')))
    })
  }
  async function health() {
    assertServicesAlive(children)
    const r = await fetch('http://127.0.0.1:19443/api/v1/version'); assert.equal(r.status, 200); assert.equal((await r.json()).version, '28.2.0')
    for (const family of [4, 6]) {
      const status = await new Promise((resolve, reject) => http.get({ host: address(family), port: 19447, path: '/', timeout: 2000 }, r => { r.resume(); r.on('end', () => resolve(r.statusCode)) }).on('error', reject))
      assert.equal(status, 200)
    }
    assertServicesAlive(children)
  }
  await until(async () => { try { return (await tlsRequest(4)).status === 403 } catch { return false } }, 'verified TLS ingress')
  // Negative TLS trust check while admission is still unmanaged and healthy.
  await assert.rejects(() => tlsRequest(4, { trusted: false }), e => ['UNABLE_TO_VERIFY_LEAF_SIGNATURE', 'SELF_SIGNED_CERT_IN_CHAIN', 'UNABLE_TO_GET_ISSUER_CERT_LOCALLY'].includes(e.code))
  proof.tls = { untrustedRejected: true }
  const policy = `apiVersion: picketx.io/v1alpha1\nkind: Resource\nmetadata: {name: gitea, uid: resource-gitea-v1}\nspec: {endpoints: [{name: https, network: {protocol: TCP, port: 19445}}, {name: ssh, network: {protocol: TCP, port: 19222}}]}\n---\napiVersion: picketx.io/v1alpha1\nkind: EnforcementBinding\nmetadata: {name: gitea-host, uid: binding-gitea-v1}\nspec:\n  resources: [{resourceRef: {name: gitea, uid: resource-gitea-v1}}]\n  agents: [{agentRef: {name: node}}]\n  enforcement: {profileRef: host, class: host-local, stage: original-destination, allowedDestinations: [127.0.0.1, '::1'], handling: graceful}\n---\napiVersion: picketx.io/v1alpha1\nkind: HTTPRoute\nmetadata: {name: gitea-http, uid: route-gitea-v1}\nspec: {resourceRef: {name: gitea, uid: resource-gitea-v1}, endpointName: https, host: '${authority}', methods: [GET, POST], pathPrefixes: [/]}\n`
  await publish(policy, 'Native Gitea independent entries'); await confirmed()
  const git = async (family, endpoint, { user = 'owner', anonymous = false, knownHosts = known, clone = false } = {}) => {
    const ip = family === 4 ? '127.0.0.1' : '[::1]'
    const url = endpoint === 'https' ? `https://${ip}:19445/owner/private.git` : `ssh://nobody@${ip}:19222/owner/private.git`
    const args = ['-c', 'credential.helper=', '-c', 'http.sslVerify=true', '-c', `http.sslCAInfo=${dir}/ca.pem`, '-c', `http.extraHeader=Host: ${authority}`, '-c', 'http.connectTimeout=2', '-c', 'http.lowSpeedLimit=1', '-c', 'http.lowSpeedTime=3', ...(clone ? ['clone', '--quiet', url, join(dir, `clone-${endpoint}-${family}`)] : ['ls-remote', url, 'refs/heads/main'])]
    const env = { ...process.env, GIT_TERMINAL_PROMPT: '0', GIT_ASKPASS: askpass, GITEA_GIT_USER: anonymous ? '' : user, GITEA_GIT_TOKEN: anonymous ? '' : user === 'owner' ? token : pass,
      GIT_SSH_COMMAND: `ssh -F /dev/null -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=${knownHosts} -o ConnectTimeout=2 -i ${keys[user]}` }
    try {
      const result = await command('git', args, { env, timeout: 6000, maxBuffer: 1024 * 1024 })
      if (clone) assert.equal(run('git', ['-C', join(dir, `clone-${endpoint}-${family}`), 'rev-parse', 'HEAD']).trim(), proof.oid)
      else assert.equal(result.stdout.trim(), `${proof.oid}\trefs/heads/main`)
      return { outcome: 'allow' }
    } catch (e) {
      const message = String(e.stderr ?? '')
      if (/timed out|Failed to connect|Connection refused|Couldn't connect to server/i.test(message)) return { outcome: 'transport-deny' }
      if (/Authentication failed|could not read Username|repository.*not found|Could not read from remote repository|403|401/i.test(message)) return { outcome: 'app-deny' }
      if (/REMOTE HOST IDENTIFICATION HAS CHANGED|Host key verification failed/.test(message)) return { outcome: 'host-key-deny' }
      throw e
    }
  }
  const dead = new Set(['4-https', '4-ssh', '6-https', '6-ssh'])
  async function scenario(name) {
    await health(); const observations = []
    for (const family of [4, 6]) for (const endpoint of ['https', 'ssh']) {
      const { outcome } = await git(family, endpoint)
      assert.equal(outcome, dead.has(`${family}-${endpoint}`) ? 'transport-deny' : 'allow', `${name} ${family}/${endpoint}`)
      observations.push({ family, endpoint, outcome })
    }
    await health(); proof.cases.push({ name, revision: (await api('publication')).revision, observations }); console.log(`PASS actual Gitea ${name}`)
  }
  const grants = new Map()
  async function issue(family, endpoint) {
    await page.getByRole('button', { name: 'Grants', exact: true }).click()
    await page.getByRole('button', { name: 'Use latest Grant revision', exact: true }).click()
    await page.getByLabel('Grant name', { exact: true }).fill(`gitea-${endpoint}-${family}`)
    await page.getByLabel('Source IP or CIDR', { exact: true }).fill(address(family))
    await page.getByLabel('Resource', { exact: true }).selectOption('resource-gitea-v1')
    await page.getByRole('checkbox', { name: `${endpoint} TCP/${endpoint === 'https' ? 19445 : 19222}`, exact: true }).check()
    await page.getByLabel('Grant issue reason', { exact: true }).fill(`Native Gitea ${endpoint} IPv${family}`)
    const response = page.waitForResponse(r => r.url().endsWith('/grants') && r.request().method() === 'POST')
    await page.getByRole('button', { name: 'Issue Grant', exact: true }).click()
    const r = await response; assert.equal(r.status(), 201); const created = await r.json()
    await page.locator(`[data-grant-id="${created.uid}"]`).waitFor()
    const revision = await confirmed()
    const grant = (await api('grants?limit=100')).grants.find(g => g.uid === created.uid)
    grants.set(`${family}-${endpoint}`, grant); proof.grants.push({ family, endpoint, revision, grant }); dead.delete(`${family}-${endpoint}`)
  }
  async function revoke(family, endpoint) {
    const grant = grants.get(`${family}-${endpoint}`)
    await page.locator(`[data-grant-id="${grant.uid}"]`).getByRole('button', { name: 'Revoke Grant', exact: true }).click()
    await page.getByLabel('Revocation reason', { exact: true }).fill(`Native Gitea revoke ${endpoint} IPv${family}`)
    const response = page.waitForResponse(r => r.url().endsWith(`/grants/${grant.uid}/revoke`))
    await page.getByRole('button', { name: 'Confirm revocation', exact: true }).click(); assert.equal((await response).status(), 200)
    await confirmed(); dead.add(`${family}-${endpoint}`)
  }
  let appBrowser, nssLabel
  const keep = new https.Agent({ keepAlive: true, maxSockets: 1, ca })
  try {
    await scenario('baseline')
    for (const family of [4, 6]) await issue(family, 'https')
    await scenario('httpsOnly')
    for (const family of [4, 6]) await issue(family, 'ssh')
    // Public evidence canonical order is family, endpoint, not execution order.
    proof.grants.sort((a, b) => a.family - b.family || (a.endpoint === 'https' ? -1 : 1))
    await scenario('both')
    for (const family of [4, 6]) {
      await health()
      for (const options of [{ anonymous: true }, { user: 'outsider' }]) assert.equal((await git(family, 'https', options)).outcome, 'app-deny')
      assert.equal((await git(family, 'ssh', { user: 'outsider' })).outcome, 'app-deny')
      for (const endpoint of ['https', 'ssh']) assert.equal((await git(family, endpoint, { clone: true })).outcome, 'allow')
      await health()
      proof.applicationAuth.push({ family, anonymousHTTPS: 'denied', outsiderHTTPS: 'denied', outsiderSSH: 'denied', ownerHTTPS: 'allow', ownerSSH: 'allow', clonedOID: proof.oid })
    }
    assert.equal((await git(4, 'ssh', { knownHosts: wrongKnown })).outcome, 'host-key-deny'); proof.ssh = { wrongHostKeyRejected: true }
    const nss = join(homedir(), '.pki/nssdb'); await mkdir(nss, { recursive: true })
    try { await readFile(join(nss, 'cert9.db')) } catch { run('certutil', ['-N', '--empty-password', '-d', `sql:${nss}`]) }
    nssLabel = `picketx-native-${randomBytes(8).toString('hex')}`
    run('certutil', ['-A', '-d', `sql:${nss}`, '-n', nssLabel, '-t', 'C,,', '-i', join(dir, 'ca.pem')])
    appBrowser = await chromium.launch({ headless: true, args: ['--host-resolver-rules=MAP gitea.example 127.0.0.1', '--no-proxy-server'] })
    const appPage = await appBrowser.newPage({ viewport: { width: 1440, height: 1000 } })
    assert.equal((await appPage.goto(`https://${authority}/user/login`)).status(), 200)
    await appPage.locator('#user_name').fill('owner'); await appPage.locator('#password').fill(pass)
    await Promise.all([appPage.waitForURL(url => url.pathname !== '/user/login'), appPage.locator('form button[type=submit]').click()])
    assert.equal((await appPage.goto(`https://${authority}/owner/private`)).status(), 200)
    await appPage.getByRole('link', { name: 'private', exact: true }).first().waitFor()
    for (const width of [375, 1440]) { await appPage.setViewportSize({ width, height: 1000 }); await appPage.screenshot({ path: join(evidence, `gitea-repository-${width}.png`), fullPage: true }) }
    proof.browser = { login: true, repository: true }
    await appBrowser.close(); appBrowser = undefined
    const before = await tlsRequest(4, { agent: keep }); assert.equal(before.status, 200)
    await revoke(4, 'https')
    const hits = await upstreamHits(), after = await tlsRequest(4, { agent: keep })
    assert.equal(after.socket, before.socket, 'same established TLS socket'); assert.equal(after.status, 403)
    assert.equal(await upstreamHits(), hits, 'revoked request must not reach Gitea')
    proof.establishedHTTPS4 = { revision: (await api('publication')).revision, sameSocket: true, before: before.status, after: after.status, upstreamAfter: 0 }
    await scenario('revokeHTTPS4')
    await revoke(4, 'ssh'); await scenario('revokeSSH4')
    await revoke(6, 'https'); await scenario('revokeHTTPS6')
    await revoke(6, 'ssh'); await scenario('revokeSSH6')
    proof.healthyControls = true
    return proof
  } finally {
    keep.destroy(); await appBrowser?.close()
    if (nssLabel) run('certutil', ['-D', '-d', `sql:${join(homedir(), '.pki/nssdb')}`, '-n', nssLabel])
  }
}
