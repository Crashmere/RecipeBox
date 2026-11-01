# Local Deployment and Rollback

This project is for personal use: validate the current change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletions, and backup restoration specifically against an isolated copy first.

GitHub is only a backup of source code and configuration, and Actions is disabled. Committing and pushing do not update production. See [server-operations local deployment](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/release.md) for the shared implementation and environment preparation; this project's parameters come from `deploy/release.json`.

## Routine Use

`make deploy` targets `nas` and its dedicated deployment key. `vps` provides public HTTPS on port 8443; temporary `ali` access forwards to the same NAS application. Public-check overrides and host-specific keys are maintained in the shared release guide. Explicit legacy ALI deployment must not create a second writer after migration.

```sh
# Exercise the current feature locally, then commit and push the source.
git add <files-changed-in-this-task>
git commit -m "Describe the change"
git push origin main
make release   # Build from the committed snapshot; do not run the full test suite
make deploy    # Reuse the artifact from the same commit and deploy it over SSH
```

Local materials are under `.local/releases/<full-commit>/` and contain `program`, `manifest.json`, and `portal.json` from the same commit. Deployment verifies hashes and confirms that the commit is backed up to `origin/main`; it reuses the artifact when source is unchanged. Untracked files do not enter the build. `make releases` lists local versions. These materials are not committed to Git.

Use `make portal` to update only portal information. It preflights and synchronizes the declaration from the same commit without building or restarting the business program. Failure to synchronize the declaration does not automatically revert a program update that already succeeded; fix the issue and retry the declaration separately.

## Deployment Boundaries

Use the restricted `recipebox-deploy` identity; its private key exists only on the maintenance computer. For initial account setup, use `deploy/setup-deploy.sh <local-deployment-public-key>`; do not initialize an existing account again. The forced command accepts only `deploy`, `portal-check`, and `portal`, and sudo permits only this application's fixed deployment script.

Switching begins only after upload and verification succeed. On `nas`, upgrade protection must cold-copy complete business data and recovery-critical private configuration, including recipe and cooking-log media, to root-private `/var/lib/server-upgrades/recipebox/<release>/`. Candidate checks use an independent copy. Successful runtime and previous SQLite-row/file preservation checks immediately delete only this upgrade's local recovery copy; failure or uncertainty retains it. `previous` remains the program rollback artifact. Normal deployments do not transfer production data. Administrators install units, Nginx, env, authorization, and scripts from source; documentation uses `SYNC_HOST=nas` with the shared `sync-docs.sh`. Apply the shared [data-protection scope](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md#data-protection-scope) before capture.

`recipebox-cleanup.service`/`.timer` are separately installed runtime configuration and must be registered in `deploy/portal.json`. Only a host marked `nas` enables cleanup scheduling; the native backup timer remains disabled. A program-only deployment does not install or change these units. During existing-data migration, install the reviewed units without running `init`, and verify their lock/sandbox contract before authorized activation; see [OPERATIONS.md](OPERATIONS.md).

## Rollback and Troubleshooting

```sh
make releases
make rollback COMMIT=<explicitly-selected-full-old-commit>
```

Rollback reuses a retained, verified local artifact and still performs backup and health checks. First confirm that the old program is compatible with current data. Program rollback does not restore the database, photos, or device state. To restore data, first choose a recovery point and preserve the current state, then follow OPERATIONS/RESTORE.

Cooking-log photos add an optional field to existing record JSON without a schema migration. Once logs contain photos, releases predating this feature cannot safely manage their references: editing a log or saving its recipe can detach or discard them. Use a release that understands cooking-log photos for rollback of a populated database.

NAS retains the program, `previous`, metadata, result/recovery, and upgrade-acceptance evidence under `/opt/recipebox/releases/`; `current-commit` records the actual running source. `.local/releases/<commit>/last-deployment.json` records the local result. After a failed switch, inspect the result, journal, and retained private snapshot. NAS does not automatically prune uncertain upgrade copies. [Legacy automatic retention](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md) applies to `ali`, not the NAS recovery namespace.
