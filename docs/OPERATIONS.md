# Installation and Operation

The public entry point uses HTTPS with a trusted IP certificate, preserving the existing `/recipebox/` path. Public HTTP returns 308; API clients use HTTPS directly. Nginx overwrites `X-Forwarded-Proto`; write-origin validation trusts proxy headers only from loopback addresses and still rejects cross-site origins. See [shared HTTPS operations](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/https.md) (server copy: `/opt/server-context/references/https.md`) for certificates, renewal, rollback, and whole-server acceptance. This project's backend and deployment checks retain local HTTP. The proxy-inspection entry point at `127.0.0.1:80` is not publicly reachable. Public access uses ServerPortal's shared device authentication: enter the passphrase at `/portal/login` to authorize a device, then use the same-origin Secure/HttpOnly Cookie. Unauthorized APIs return 401. Local deployment checks and service-to-service calls remain available.

Before operating `ali`, load server-operations and explicitly read `/opt/AGENTS.md`. Do not record the real public address in the public repository; obtain live information through the trusted SSH alias.

Image sizes: Nginx permits requests up to 26 MiB, and the program permits each image up to 25 MiB. If an image over 1 MiB returns 500 before reaching the program, see [shared authentication incorrectly blocks large requests](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#shared-authentication-incorrectly-blocks-large-requests). The shared fix preserves this application's upload limit.

## Layout

`/opt/recipebox/bin/recipebox` is the Linux amd64 program with embedded pages. `config` contains this project's units/location; `data` contains `recipebox.db`, `media`, and `tmp`; `backups` contains snapshots; `docs` and `AGENTS.md` are controlled documentation copies; `releases` retains deployment history; and `current-commit` identifies the program source.

The runtime identity is `recipebox`, and the deployment identity is `recipebox-deploy`. `data`/`backups` use 0700 and files use 0600; root manages programs and configuration. The service listens on `127.0.0.1:18083` and is reached through shared Nginx under `/recipebox/`, without adding a public listening port. systemd limits writes to `data`, memory to 640 MiB, and CPU to 100%.

## Initial Installation

First check the port, identities, directories, and health of all existing services. Build Go locally. The server needs only `libvips-tools` and `libheif-plugin-libde265` (the existing official Ubuntu packages on this host); do not install Go, Node, Docker, or a database service. Follow software-installation for software installation.

```sh
make linux
# Upload deploy files from the reviewed commit and the verified binary to an independent staging directory.
bash deploy/install.sh /path/to/recipebox-linux-amd64
ln -s /opt/recipebox/config/nginx-location.conf /etc/nginx/app-locations/recipebox.conf
nginx -t
systemctl reload nginx
```

`install` refuses to overwrite existing directories/identities, explicitly initializes an empty production database, starts the service and backup timer, and obtains the first backup. Do not put test records in the production database. After installation, check direct and proxied access for all four services, deep links, and assets.

## Local Development and Browser Verification

README contains startup steps. When the current Mac has the goenv wrapper, use `GOENV_VERSION=1.24.0 go ...`; `go.mod` selects `go1.26.8`. Do not change the system default version. Run `init` only once; use `serve` directly for an existing database.

```sh
npm --prefix web run build
bin/recipebox init --data .local/e2e-data
bin/recipebox serve --data .local/e2e-data --with-prefix
# In another terminal:
```

To test an empty database, create another uniquely named directory; do not clear production data.

## Backup and Recovery

Run daily at 03:45 Asia/Shanghai with a randomized delay of 0–5 minutes, retaining 14 `daily` backups. The backup unit's writable directory must be all of `/opt/recipebox`; see [photo hard-link backups fail in a systemd sandbox](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#photo-hard-link-backups-fail-in-a-systemd-sandbox) for the reason. A backup contains a consistent SQLite snapshot, photo hard links, and a SHA-256 manifest. `before-deploy` rotates under the shared deployment-retention policy; `manual` is not cleaned automatically. On 2026-09-27, an all-application archive containing this project's database and photos was separately downloaded to the maintenance computer and verified; see the [shared backup description](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md#manual-data-archives). This application's scripts do not proactively synchronize off-host. The server separately uses Alibaba Cloud file backup; see [host cloud backup](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md#cloud-backup) for its scope, 30-day retention, and recovery limits.

```sh
systemctl status recipebox-backup.timer
systemctl start recipebox-backup.service
journalctl -u recipebox-backup.service -n 50 --no-pager
runuser -u recipebox -- /opt/recipebox/bin/recipebox backup --data /opt/recipebox/data --out /opt/recipebox/backups/manual-UNIQUE
```

The backup destination directory must not exist. Retry later on a lock conflict. Do not directly copy the active WAL main file or modify hard-linked photos. Only a directory with a complete `manifest.json` may be restored.

```sh
recipebox restore --source /path/to/backup --out /path/to/new-restore-directory
recipebox check --data /path/to/new-restore-directory
# First confirm that port 19083 is free, then start the isolated recovery instance
recipebox serve --data /path/to/new-restore-directory --listen 127.0.0.1:19083 --with-prefix
```

`restore` verifies hashes and relative paths, rejects an existing target, and copies photos into independent files. Before production recovery, confirm the recovery point and possible loss of later changes, stop this application, preserve the current directory, restore into a new directory, verify it, then switch identity/path and restart for validation. Program rollback is not database rollback.

## Diagnostics

```sh
systemctl status recipebox recipebox-backup.timer
journalctl -u recipebox -n 100 --no-pager
curl --fail http://127.0.0.1:18083/healthz
curl --fail http://127.0.0.1/recipebox/healthz
systemctl show recipebox -p MemoryCurrent -p MemoryPeak -p NRestarts
df -h /opt/recipebox
du -sh /opt/recipebox/data /opt/recipebox/backups
vips --version
```

Every record write emits one journal line, `write <path> action=… status=… replay=… <server-duration>`, without form contents. Format and JSON errors are rejected during parsing and are not logged. When a save is slow, first correlate this line by time. A duration of only a few milliseconds followed by `replay=true` means the server already wrote the change and the response was lost or delayed in the network before the same request arrived again. The shared Nginx access-log time is the request completion time, not a duration.

```sh
journalctl -u recipebox --since '-1h' --no-pager | grep ' write '
```

For a missing or damaged database, inspect backups and permissions; do not use `init` to create an empty database and conceal the error. For 507, check root-disk and photo quotas; for 429, check the backup lock and concurrent uploads; for 422, check image format and vips logs. libvips 8.15 and 8.18 both use `--export-profile=srgb`.

After committing and pushing documentation, run `~/agent-config/skills/server-operations/scripts/sync-docs.sh RecipeBox`. It handles drift checks, installation under `/opt/recipebox`, per-file verification, `docs/SOURCE`, and cleanup; see server-operations maintenance for usage. After changing the shared inventory, run the same script without arguments.

## ServerPortal Integration Materials

`deploy/portal.json` is the maintained source for this application's resource description. Local deployment validates it with the server-operations validator, then stores the same declaration and binary in the same local version directory. It runs `portal-check` before deployment and `portal` afterward, using the existing restricted SSH identity to install `/opt/recipebox/config/portal.json` and verify the SHA-256 actually loaded by the collector. `config/portal-source.json` records the declaration's source commit; it and the program's `current-commit` identify versions of different materials.

The portal discovers this application through the controlled link `/opt/serverportal/registry.d/recipebox.json`. It automatically reloads a successfully updated declaration without a restart. The first normal local deployment also creates the link, so there is no separate central application list to edit. An ordinary deployment may update this application's declaration; an administrator still installs other units, env files, Nginx configuration, and deployment scripts.

Source-code or database-behavior changes cannot use this option as a substitute for program deployment.

When data roots, media, backup formats, units, ports, or access paths change, update the declaration and corresponding documentation in the same commit, update the shared inventory, and check resource coverage. The portal reads files, media, database tables, and systemd state automatically; a maintenance agent must explicitly update directory purposes, API descriptions, and permission boundaries. See [portal maintenance](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/portal.md) for shared protocols, failure handling, and new-application onboarding.

The portal at `/portal/` protects public access. Deployment scripts check application health through loopback, and local deployment's public check expects 401 for unauthorized access. Device authorization remains valid until explicitly revoked; shared Nginx renews the Cookie on valid requests. If this application adds an `add_header`, it must preserve shared `Set-Cookie` forwarding; see shared portal maintenance for rules and acceptance. Portal backups use this application's native consistent snapshot. At the user's request, real full-chain recovery acceptance remains deferred and must not automatically resume downloads or restoration during this maintenance.

## Phone Home-Screen Icon

The current `/recipebox/apple-touch-icon.png` is 180×180. Nginx rules allow anonymous GET/HEAD only for it, `favicon.ico`, and `recipebox.svg`. Pages, APIs, and user media remain protected by device authentication. See the [shared troubleshooting record](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#missing-iphone-home-screen-icons-after-shared-authentication) for the common cause, deployment status, and phone acceptance.

## Current Release Entry Point

This project is for personal use: validate the current change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletions, and backup restoration specifically against an isolated copy first.

See [Local Deployment and Rollback](DEPLOYMENT.md) for the complete procedure. GitHub stores only source code. Local `make release` builds, `make deploy` updates production, and documentation is synchronized separately.

## Automatic Cleanup of Deployment Materials

Every day at 05:00 Beijing time, the server follows [automatic retention of deployment materials](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md) to retain the latest 3 successful deployments and latest 5 complete pre-deployment backups, while protecting the current version, its corresponding backup, and failed batches pending review. server-operations owns the shared implementation, lock, receipts, preview, and disable command. This project's `deploy` retains the deployment script's `recovery` marker and the daily backup's `flock` entry point. On first installation, create `/run/lock/ali-release-retention.lock` under the shared documentation before enabling the backup timer. `daily`, `manual`, business data, portal `exports`, and build materials on the maintenance computer are outside this automatic cleanup scope.
