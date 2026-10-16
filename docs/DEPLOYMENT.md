# Local Deployment and Rollback

This project is for personal use: validate the current change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletions, and backup restoration specifically against an isolated copy first.

GitHub is only a backup of source code and configuration, and Actions is disabled. Committing and pushing do not update production. See [server-operations local deployment](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/release.md) for the shared implementation and environment preparation; this project's parameters come from `deploy/release.json`.

## Routine Use

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

Switching begins only after upload completion and successful verification. Preserve the pre-upgrade backup and `previous`, and restore the old program if health checks fail. Normal deployments do not carry production data. An administrator explicitly synchronizes units, Nginx, env, authorization, and deployment scripts from source; documentation uses the shared `sync-docs.sh`.

## Rollback and Troubleshooting

```sh
make releases
make rollback COMMIT=<explicitly-selected-full-old-commit>
```

Rollback reuses a retained, verified local artifact and still performs backup and health checks. First confirm that the old program is compatible with current data. Program rollback does not restore the database, photos, or device state. To restore data, first choose a recovery point and preserve the current state, then follow OPERATIONS/RESTORE.

The server retains program, `previous`, `metadata`, `result`, and `recovery` under `/opt/recipebox/releases/`; `current-commit` records the actual running source. `.local/releases/<commit>/last-deployment.json` records the local deployment result. An upload failure normally occurs before the service is stopped; for a failure after switching, inspect `result` and the journal first. Server deployment history and ordinary pre-deployment snapshots are automatically rotated under [automatic retention of deployment materials](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md). Local builds, `manual` and migration backups, and portal export packages are excluded.
