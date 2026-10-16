# RecipeBox Maintenance Entry Point

Write agent instructions, maintenance documentation, technical plans, handoffs, and similar agent-authored operational documents in English by default unless the user explicitly requests another language. User-facing product output follows the language requested by the user.

This project is for personal use: validate the current change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletions, and backup restoration specifically against an isolated copy first.

GitHub backs up only source code and configuration; pushing does not trigger tests or deployment. See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for local entry points and rollback; shared procedures are maintained by server-operations.

Read [docs/README.md](docs/README.md) first, then read architecture, API, operations, and verification documentation as the task requires.

- Family-shared recipes use shared device authentication. Authorized devices can view, add, modify, delete, and export. Recipes are the primary entity; pantry items are a separate supplementary feature.
- Go embeds Vue and uses SQLite plus libvips. The server is the authoritative data source; browser drafts are not backups.
- Record writes use `revision` and `Idempotency-Key`. Ordinary profile edits must not clear cooking logs, and consuming a pantry item must not overwrite other fields.
- Photos are immutable, and the database and photos are backed up together. Startup must report a missing database as an error and must not initialize an empty database implicitly.
- Tests use isolated `.local` data. Production acceptance is read-only and does not import fictional family recipes.
- Before deployment, read server-operations and explicitly run `ssh ali 'cat /opt/AGENTS.md'`. Keep source, deployment configuration, and current documentation consistent, then, after pushing, run agent-config's `skills/server-operations/scripts/sync-docs.sh RecipeBox` to synchronize the server copies.
- Use only the authorization table in server-operations `SKILL.md` to decide what to finish and report afterward versus what to confirm first. Documentation maintenance and synchronization do not require advance confirmation.
- The repository is public. Do not commit the production database, photos, backups, credentials, or the server's public address.

## Portal Resource Synchronization

- When changing website icons or authentication, also check the iPhone `apple-touch-icon`, its post-build path, and anonymous GET/HEAD access. Only explicit brand icons and a public manifest may be exceptions; pages, APIs, and user media must remain authenticated. See server-operations common issues for shared troubleshooting and acceptance; current-state is authoritative for actual enablement status.

- This project's `deploy/portal.json` is the maintained source for the ServerPortal resource declaration. It records directory purposes, the database, runtime user, port, units, access paths, APIs, and backup types. When adding, migrating, or deleting a data root, interface, or runtime material, update the declaration, the corresponding docs, and the shared application inventory together.
- The declaration is deployed to `/opt/recipebox/config/portal.json` and managed by root. Local deployment uses the shared server-operations validator: it performs a preflight before deployment, then automatically synchronizes over restricted SSH and verifies the hash loaded by the portal. `registry.d/recipebox.json` registers the link automatically; updates do not require restarting the portal.
- Shared Nginx and the portal provide unified authentication; this project does not maintain a separate device allowlist. Local calls and deployment health checks remain available under the shared conventions. Device authentication is enabled in production; synchronize server-operations current-state after changes.
- The portal's read-only display does not replace this project's native consistent backups. When backup formats or media lifecycles change, validate the affected backup and recovery contracts against an isolated copy. Real business data, credentials, and backups still must not enter Git.

Local deployment automatically preflights and synchronizes the `deploy/portal.json` from the same commit. To update only the declaration, run `make portal`; this preserves the business-program version.

Web pages block ordinary Tab / Shift+Tab control traversal and remove focus highlights by default; retain only exceptions explicitly requested by the user. Maintain and verify this behavior under the [server-operations web keyboard and focus conventions](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/conventions.md#web-keyboard-and-focus).
