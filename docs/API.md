# HTTP API

The external prefix is `/recipebox/`, which Nginx removes. The following are internal paths of the Go service; `{id}` and `{key}` are path parameters. Public APIs and media are protected by ServerPortal's shared device authentication and return 401 when unauthorized; local loopback calls remain available. JSON uses camelCase. Every POST uses a 32-character lowercase hexadecimal `Idempotency-Key`. Reuse of a key for record writes must preserve the HTTP method, internal path, and body bytes; reuse for an upload must preserve the original file bytes. Changes to an existing record must include the current `revision`.

| Method and path | Purpose |
| --- | --- |
| GET /healthz | Check database-connection health |
| GET /api/records | Return an unpaginated array of all non-deleted records; query parameter `trash=1` returns the recycle bin; sorted by `updatedAt`, then `id`, descending |
| GET /api/records/{id} | Return one record, including `deletedAt`; records still retained in the recycle bin can also be read |
| POST /api/records | Create a record; `kind=recipe/pantry` and `name` are required; `action` is omitted or `save`; returns 200 and a Record |
| POST /api/records/{id} | Body includes the current `revision`; `action` is `save`/`favorite`/`consume`/`log`/`remove_log`/`delete`/`restore`; success returns 200 and the complete Record |
| GET /api/operations/{key} | Return the result of the original record write or upload; unknown keys return 404, which does not describe the resource's current state |
| POST /api/uploads | Original image file as the body (not multipart), maximum 25 MiB; returns 201 and Media |
| GET /api/media/{id}/main | Standard JPEG, available only for a valid staged photo or one still referenced by a non-deleted record |
| GET /api/media/{id}/thumb | WebP thumbnail, with the same ownership and validity checks as `main` |
| GET /api/export | ZIP containing all non-deleted records, with `recipes-and-pantry.json` and `photos/{id}.jpg`; excludes the recycle bin |

This table covers the current business, health, and media interfaces; keep `deploy/portal.json.apis` consistent with it. Register each method and path only once. The `action` values below are JSON fields of the same record-write interface, not separate paths or query parameters. See the [shared troubleshooting guidance](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#portal-api-list-does-not-match-the-service) for checking and synchronization procedures.

## Record Write Actions

The JSON body of `POST /api/records/{id}` always includes the current `revision`, for example `{"revision":3,"action":"favorite","favorite":true}`.

| `action` | Input and behavior |
| --- | --- |
| `save` (default) | Full profile; preserves the original `kind`, creation time, and cooking logs. Omitted editable fields use their zero values; this is not a partial update |
| `favorite` | Boolean `favorite`; only recipes can be favorited |
| `consume` | Decrement a pantry item's `quantity` by one; no stock or the wrong kind returns 409 |
| `log` | `log={id?,date,note,rating}`; recipes only; without `id`, add a log; with `id`, update the existing log |
| `remove_log` | `log.id` selects the cooking log to delete |
| `delete` | Soft-delete into the recycle bin |
| `restore` | Restore a record deleted within 30 days; a non-deleted record returns 409 and an expired record returns 410 |

Every successful action increments `revision` and returns the complete Record. Initial creation can use only `save`, and an existing record cannot change `kind`. JSON forms are limited to 256 KiB and reject unknown fields and trailing JSON. Uploads support JPEG, PNG, WebP, HEIC, and HEIF. Media has the shape `{id,width,height,bytes,createdAt}`. A staged photo must be attached to a record within 24 hours, and one photo can belong to only one record. Variants other than `main`/`thumb`, removed photos, photos on deleted records, and expired staged photos all return 404.

A record contains `id`/`kind`/`revision`/`name`/`category`/`notes`/`tags`/`ingredients`/`steps`/`minutes`/`servings`/`favorite`/`logs`/`photoIds`/`quantity`/`unit`/`location`/`purchaseDate`/`expiryDate`/`createdAt`/`updatedAt`/`deletedAt`. Canonical definitions are in `internal/app/model.go` and `web/src/types.ts`.

Application errors are JSON `{code,message}`: 400 for format/submission-key errors; 403 for origin; 404 for missing resources; 409 for version conflict, key reuse, or no stock; 410 for an expired restore window; 413 for size; 415 for format; 422 for content validation/image decoding; 429 for busy/rate limit; 500 for internal errors; and 507 for capacity. The shared public entry point returns 401 for authentication failure. Per IP, write operations are limited to 120 per minute and uploads separately to 30; only `X-Real-IP` from a loopback proxy is accepted. No response does not mean the operation was not committed: query the original key or retry the original bytes. After a write has no response for 3 seconds, the web page automatically queries the original key. Operation results are retained by maintenance for about 7 days; an older unknown submission must not be retried automatically.
