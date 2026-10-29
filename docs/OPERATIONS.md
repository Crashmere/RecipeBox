# Installation and Operation

The public entry point uses HTTPS with a trusted IP certificate, preserving the existing `/recipebox/` path. Public HTTP returns 308; API clients use HTTPS directly. Nginx overwrites `X-Forwarded-Proto`; write-origin validation trusts proxy headers only from loopback addresses and still rejects cross-site origins. See [shared HTTPS operations](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/https.md) (server copy: `/opt/server-context/references/https.md`) for certificates, renewal, rollback, and whole-server acceptance. This project's backend and deployment checks retain local HTTP. The proxy-inspection entry point at `127.0.0.1:80` is not publicly reachable. Public access uses ServerPortal's shared device authentication: enter the passphrase at `/portal/login` to authorize a device, then use the same-origin Secure/HttpOnly Cookie. Unauthorized APIs return 401. Local deployment checks and service-to-service calls remain available.

RecipeBox's application target is `nas`; `vps` exposes public HTTPS port 8443 and temporary `ali` access forwards to the same instance. NAS provides local-CA HTTPS for home devices. Read server-operations and the target's remote `/opt/AGENTS.md`; current-state records activation. Keep addresses in trusted SSH configuration, not this public repository.

Image sizes: Nginx permits requests up to 26 MiB, and the program permits each image up to 25 MiB. If an image over 1 MiB returns 500 before reaching the program, see [shared authentication incorrectly blocks large requests](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#shared-authentication-incorrectly-blocks-large-requests). The shared fix preserves this application's upload limit.

## Layout

`/opt/recipebox/bin/recipebox` is the Linux amd64 program with embedded pages. `config` contains this project's units/location; `data` contains `recipebox.db`, `media`, and `tmp`; `backups` preserves legacy native snapshots without NAS scheduling; `docs` and `AGENTS.md` are controlled documentation copies; `releases` retains program history without NAS scheduled pruning; and `current-commit` identifies the program source. NAS upgrade protection instead lives in root-private `/var/lib/server-upgrades/recipebox/<release>/`.

The runtime identity is `recipebox`, and the deployment identity is `recipebox-deploy`. `data`/`backups` use 0700 and files use 0600; root manages programs and configuration. The service listens on `127.0.0.1:18083` and is reached through shared Nginx under `/recipebox/`, without adding a public listening port. systemd limits writes to `data`, memory to 640 MiB, and CPU to 100%.

## Application Cleanup

The NAS migration separates existing application cleanup from the disabled native daily backup job. Its dedicated units are `recipebox-cleanup.service` and `recipebox-cleanup.timer`; scheduling is enabled only on a host marked `nas`. The timer retains 03:45 Asia/Shanghai with up to five minutes of randomized delay and persistent catch-up. The [shared inventory](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md) records actual installation and activation.

The cleanup service runs `recipebox cleanup --data /opt/recipebox/data` as `recipebox`, with writes limited to `data`, a 256 MiB memory limit, 50% CPU quota, Nice 10, and a 15-minute timeout. It takes the shared maintenance lock, then the exclusive application deployment lock before applying the existing deleted-record/media/operation cleanup rules. Shared `/etc/tmpfiles.d/server-maintenance.conf` prepares these root-owned 0644 lock files before the units run. Portal registers these maintenance units so full cold backups coordinate their writer pause with the same locks. This operation generates no snapshot and prunes no backup or release material; native backup timers remain disabled on NAS.

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

`install` refuses existing directories/identities, explicitly initializes an empty database, starts the service, and leaves native backup scheduling disabled by default. It is not a migration importer; migration instead transfers verified complete cold state while the old service is paused. Do not put test records in production. Check direct/proxied health, deep links, and assets after installation.

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

NAS routine backups are manual complete compressed archives on an explicitly identified external volume through Portal or the agent. Pause business/background services, cold-copy all data/config/program state including recipe and cooking-log media to external staging, resume previously running services, then compress and verify the immutable copy. No routine stage/archive, native daily timer, or automatic backup deletion runs on NAS. The verified 2026-09-27 maintenance-computer archive and independent ALI Cloud Backup remain preserved in shared current-state.

Native compatibility checks use an isolated data copy:

```sh
systemctl status recipebox-backup.timer
journalctl -u recipebox-backup.service -n 50 --no-pager
runuser -u recipebox -- /opt/recipebox/bin/recipebox backup --data /path/to/isolated-data --out /path/to/new-native-snapshot
```

The native command remains for old packages and isolated checks: consistent SQLite, photo hard links, and a SHA-256 manifest. Its destination must not exist; lock conflicts fail. Native packages are not the NAS full-state job. Do not copy an active WAL main file or modify hard-linked photos. A native restore requires a completed `manifest.json`.

```sh
recipebox restore --source /path/to/backup --out /path/to/new-restore-directory
recipebox check --data /path/to/new-restore-directory
# First confirm that port 19083 is free, then start the isolated recovery instance
recipebox serve --data /path/to/new-restore-directory --listen 127.0.0.1:19083 --with-prefix
```

The native `restore` verifies hashes/relative paths and copies photos independently into a new target. Full NAS cold archives are instead extracted and verified in an isolated directory before the agent restores the complete application subtree. Confirm the recovery point and later data loss; stop writes, preserve current state, set ownership, and restart for read-only validation. Keep NAS native scheduling disabled. Program rollback is not database restoration.

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

After pushing managed documentation, run `SYNC_HOST=nas ~/agent-config/skills/server-operations/scripts/sync-docs.sh RecipeBox`; it verifies drift/files and records `docs/SOURCE`. Shared gateway context synchronizes to its own hosts.

## ServerPortal Integration Materials

`deploy/portal.json` is the maintained source for this application's resource description. Local deployment validates it with the server-operations validator, then stores the same declaration and binary in the same local version directory. It runs `portal-check` before deployment and `portal` afterward, using the existing restricted SSH identity to install `/opt/recipebox/config/portal.json` and verify the SHA-256 actually loaded by the collector. `config/portal-source.json` records the declaration's source commit; it and the program's `current-commit` identify versions of different materials.

The portal discovers this application through the controlled link `/opt/serverportal/registry.d/recipebox.json`. It automatically reloads a successfully updated declaration without a restart. The first normal local deployment also creates the link, so there is no separate central application list to edit. An ordinary deployment may update this application's declaration; an administrator still installs other units, env files, Nginx configuration, and deployment scripts.

Source-code or database-behavior changes cannot use this option as a substitute for program deployment.

When data roots, media, backup formats, units, ports, or access paths change, update the declaration and corresponding documentation in the same commit, update the shared inventory, and check resource coverage. The portal reads files, media, database tables, and systemd state automatically; a maintenance agent must explicitly update directory purposes, API descriptions, and permission boundaries. See [portal maintenance](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/portal.md) for shared protocols, failure handling, and new-application onboarding.

Portal protects both HTTPS entrances; loopback release checks remain private and public unauthorized APIs return 401. Device grants persist until revocation; preserve shared cookie-renewal headers. NAS Portal backups cold-copy complete state to external staging, not the narrower native media snapshot. Recovery remains agent-operated after confirming the recovery point.

## Phone Home-Screen Icon

The current `/recipebox/apple-touch-icon.png` is 180×180. Nginx rules allow anonymous GET/HEAD only for it, `favicon.ico`, and `recipebox.svg`. Pages, APIs, and user media remain protected by device authentication. See the [shared troubleshooting record](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#missing-iphone-home-screen-icons-after-shared-authentication) for the common cause, deployment status, and phone acceptance.

## Current Release Entry Point

This project is for personal use: validate the current change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletions, and backup restoration specifically against an isolated copy first.

See [Local Deployment and Rollback](DEPLOYMENT.md) for the complete procedure. GitHub stores only source code. Local `make release` builds, `make deploy` updates production, and documentation is synchronized separately.

## Deployment-Material Cleanup

NAS has no daily backup/release pruning. Successful upgrade acceptance removes only its private `/var/lib/server-upgrades/recipebox/<release>/` and candidate work copy; failed or uncertain copies remain for the agent. Existing ALI and maintenance-computer backups are untouched. [Legacy retention](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md) is not enabled for NAS upgrades.
