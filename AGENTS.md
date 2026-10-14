# PicketXD development

This repository contains Linux Agent validation, licensed GPL-3.0-or-later. Controller/shared Core belong to the separate Apache-2.0 PicketX repository.

- English documentation is default; synchronize Chinese counterparts.
- M0 inputs under internal/validation are private test fixtures, not manifest/API schemas or a production Grant compiler.
- IPv4 and IPv6 have equal coverage. Preserve permission union, explicit deny precedence, and absolute deadlines.
- Runtime backends use Go Netlink libraries; no mandatory C library or nft/conntrack command.
- Kernel tests must run inside disposable network namespaces, own only table inet picketx/picketx_ objects, and never flush the entire conntrack table.
- Apply and confirm nftables before scoped strict revocation. Preserve unmarked, unrelated and independently allowed flows. Interrupted dumps, partial deletes and remaining targets are errors.
- PICKETX_KERNEL_TESTS=1 is explicit opt-in. Opted-in setup/privilege failure must fail, not skip. A skipped suite never completes M0.
- Record actual kernel/toolchain/module versions and packet observations. Do not claim compatibility from mocks or unexecuted tests.
- Use constructor injection and standard slog for future runtime implementation. Controller transport, event subscription, activity renewal and stable business/config schemas are outside M0.
