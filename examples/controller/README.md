# Controller host enforcement

Build the static Agent as described in the main README. Run the separate Core
Controller with an enrolled Agent Token and an authoritative publication selecting
this Agent, host profile and destination scope. Keep the Controller endpoint outside
the protected application ports. Store the raw Token in a private, owned regular file
(mode 0600); rotate that file atomically and let the stream reconnect.

```sh
sudo picketxd enforce \
  --controller controller.example:9443 \
  --controller-server-name controller.example --controller-ca /etc/picketx/ca.pem \
  --agent-token-file /etc/picketx/agent.token \
  --provider controller:production --agent node --profile host \
  --state-dir /var/lib/picketx \
  --destination 192.0.2.100 --destination 2001:db8::100 \
  --mark-first 0x504a0000 --mark-last 0x504a003f \
  --handling strict --poll 5s
```

Use real local destination addresses and an operator-reserved mark range. TLS
validates hostname and CA; omit --controller-ca for system roots. To deliberately
use plaintext instead, remove TLS flags and specify --controller-plaintext. Plaintext
does not protect the bearer Token in transit. No mTLS is required.

Exactly one source is allowed: --policy or --controller. Controller mode rejects
--once. It never reads a static policy as a fallback. --provider is the expected
Controller authority, independent of the transport address. The cache/owner identity
also binds Agent/profile/destination/mark and transport settings. Keep these stable
across restarts; changing identity is a reviewed deployment migration, not cache adoption.

Only one host-local/original-destination context with matching local profile/handling
is supported. Remote namespace/context identifiers are not local kernel facts. Every
remote context bound and match must fit explicit local destinations. Unknown fields,
unsupported capabilities, ambiguous targets, regressed/equivocating revisions and
invalid contributions reject the whole snapshot. More complex remote inventories must
be split into supported deployments. Snapshot-derived objects still obey the shared
configuration UID and replacement constraints.

A complete empty target list withdraws protection; removing only Grants retains
protected entries with default deny. Independent absolute deadlines, deny precedence
and permission union survive replay, outage and restart. A private version-2 desired
journal stores the wire envelope and derived policy, with atomic durability behavior;
it is not compatible with earlier static-only binaries. Static version-1 caches remain
supported only in static mode. Do not copy journals across provider identities.

APPLIED is sent only after the existing driver confirms current native enforcement
and strict cleanup when selected. It is an authenticated Agent assertion, not independent
Controller proof. RECEIVED is only transport receipt. FAILED covers rejected or failed
execution. Delivery can fail after native apply, so absence of a report does not prove
absence of enforcement. The alpha protocol has no report acknowledgement or heartbeat;
a report must not be interpreted as continuing connectivity/health. Network failures
reconnect at one-second intervals, with two-second send limits; fresh valid intent is
required before a cacheless Agent claims readiness. LKG continues to enforce during
outage. A revoked Token prevents future sync but does not itself revoke cached permanent
permissions: publish and confirm restrictive intent before removing enrollment, or
use independently bounded Grants.

--systemd-notify remains optional. The shipped unit is a static example; replace its
source flags deliberately for Controller mode and preserve its other sandbox settings.
No global boot firewall guarantee or complete missed-interval durable revocation is added.
