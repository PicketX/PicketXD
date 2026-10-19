// SPDX-License-Identifier: GPL-3.0-or-later
// Native acceptance adapter only. Separate Core CLI owns private rendering;
// this disposable ingress fixture owns listeners, TLS and application forwarding.
import assert from 'node:assert/strict'
import { lstat, readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'

const hash = value => createHash('sha256').update(value).digest('hex')
const routeIDs = ['route-v1', 'route-private-v1', 'route-gitea-v1']
const hashPattern = /^[a-f0-9]{64}$/
const keys = (value, expected) => {
  assert(value && typeof value === 'object' && !Array.isArray(value), 'required ingress receipt')
  assert.deepEqual(Object.keys(value).sort(), [...expected].sort(), 'only public ingress fields')
}

export async function prepareInstalledIngress(installed) {
  assert.equal(installed.runtimeOptions.env.PATH, '')
  assert.equal(hash(await readFile(installed.cliBinary)), installed.proof.cliSHA256)
  const paths = [], routes = []
  for (const uid of routeIDs) {
    const directory = join(installed.instance, `nginx-${uid}`)
    execFileSync(installed.cliBinary, ['nginx-init', '--directory', directory,
      '--token-file', installed.ingressTokenFile, '--route-id', uid, '--authz-address', '127.0.0.1:19091'],
    { ...installed.runtimeOptions, encoding: 'utf8', timeout: 10000 })
    const info = await lstat(directory)
    assert(info.isDirectory() && !info.isSymbolicLink() && (info.mode & 0o777) === 0o700)
    const location = join(directory, 'location.conf'), server = join(directory, 'server.conf')
    const receipt = { uid, directoryMode: 0o700 }
    for (const [name, path] of [['location', location], ['server', server]]) {
      const stat = await lstat(path)
      assert(stat.isFile() && !stat.isSymbolicLink() && (stat.mode & 0o777) === 0o600)
      receipt[`${name}Mode`] = stat.mode & 0o777
      receipt[`${name}SHA256`] = hash(await readFile(path))
    }
    paths.push({ location, server }); routes.push(receipt)
  }
  return { primary: paths[0], privateRoute: paths[1], gitea: paths[2],
    proof: { cliSHA256: installed.proof.cliSHA256, runtimePATH: '', routes, servers: {}, privateDenials: [] } }
}

// Fixed disposable topology: never a general product ingress installer.
export function installedNginxConfiguration(prefix, primary, privateRoute, gitea = false) {
  const port = gitea ? 19445 : 18445, app = gitea ? 19443 : 18443, health = gitea ? 19447 : 18447
  // The fixture's mkdtemp paths are controlled, not arbitrary Nginx syntax.
  for (const path of [prefix, primary.location, primary.server, privateRoute.location, privateRoute.server]) {
    assert(/^\/[A-Za-z0-9_./-]+$/.test(path), 'safe fixture include path')
  }
  return `worker_processes 1;
pid ${prefix}/nginx.pid;
error_log stderr error;
events { worker_connections 128; }
http {
    ${gitea ? `log_format upstream '$upstream_addr';\n    access_log ${prefix}/upstreams.log upstream;` : 'access_log off;'}
    keepalive_timeout ${gitea ? 300 : 90}s;
    server { listen 127.0.0.1:${health}; listen [::1]:${health} ipv6only=on; location ${gitea ? '/' : '= /health'} { return 200 "healthy"; } }
    server {
        listen 127.0.0.1:${port}${gitea ? ' ssl' : ''};
        listen [::1]:${port}${gitea ? ' ssl' : ''} ipv6only=on;
        ${gitea ? `ssl_certificate ${prefix}/tls.pem;\n        ssl_certificate_key ${prefix}/tls.key;` : ''}
        server_name _;
        location / {
            include ${primary.location};
            proxy_pass http://127.0.0.1:${app};
            proxy_set_header Host ${gitea ? 'gitea.example:19445' : 'application.internal'};
            ${gitea ? 'proxy_set_header X-Forwarded-Proto https;' : ''}
            proxy_set_header X-PicketX-Route "";
            proxy_set_header X-PicketX-Source "";
        }
        location = /private {
            include ${privateRoute.location};
            proxy_pass http://127.0.0.1:${app};
            proxy_set_header Host private-application.internal;
        }
        include ${primary.server};
        include ${privateRoute.server};
    }
}
`
}

export function validateInstalledIngress(i, cliSHA256, confirmations) {
  keys(i, ['cliSHA256', 'runtimePATH', 'routes', 'servers', 'privateDenials'])
  assert.equal(i.cliSHA256, cliSHA256); assert.match(i.cliSHA256, hashPattern)
  assert.equal(i.runtimePATH, '')
  assert(Array.isArray(i.routes)); assert.equal(i.routes.length, 3)
  i.routes.forEach((r, index) => {
    keys(r, ['uid', 'directoryMode', 'locationMode', 'locationSHA256', 'serverMode', 'serverSHA256'])
    assert.equal(r.uid, routeIDs[index]); assert.equal(r.directoryMode, 0o700)
    for (const name of ['location', 'server']) { assert.equal(r[`${name}Mode`], 0o600); assert.match(r[`${name}SHA256`], hashPattern) }
  })
  assert.equal(new Set(i.routes.map(r => r.serverSHA256)).size, 3, 'independent fixed callback configurations')
  keys(i.servers, ['demo', 'gitea'])
  for (const value of Object.values(i.servers)) assert.match(value, hashPattern)
  assert(Array.isArray(i.privateDenials)); assert.equal(i.privateDenials.length, 4)
  i.privateDenials.forEach((o, index) => {
    keys(o, ['server', 'family', 'revision', 'status', 'appDelta', 'admittedStatus', 'healthStatus'])
    assert.equal(o.server, index < 2 ? 'demo' : 'gitea'); assert.equal(o.family, index % 2 === 0 ? 4 : 6)
    assert(Number.isSafeInteger(o.revision) && o.revision > 0)
    assert(confirmations.some(c => c.revision === o.revision && c.enforcementConfirmed), 'private denial at confirmed admitted revision')
    assert.equal(o.status, 403, 'authenticated caller forbidden from un-enrolled route')
    assert.equal(o.appDelta, 0); assert.equal(o.admittedStatus, 200); assert.equal(o.healthStatus, 200)
  })
}
