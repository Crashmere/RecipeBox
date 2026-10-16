# Verification Principles and Known Limitations

Routine validation targets only the current change and uses isolated synthetic data. See [DEPLOYMENT.md](DEPLOYMENT.md) for exact deployment and rollback. Historical regression suites and per-run case counts are retired; the old records remain available in Git history.

Image processing depends on libvips. When images or backups change, validate input formats, output readability, and database/photo consistency specifically. Real phone camera behavior, HEIC export from the photo library, and the soft keyboard require confirmation on the corresponding device; a mobile viewport on desktop does not substitute for a real-device result. Standard photos have a longest edge of 2560 px, and original photographic files are not retained.

Production uses HTTPS and shared device authentication. Daily backups are same-disk snapshots. One manual off-host archive exists, but there is no automatic off-host synchronization. Read the actual version from `current-commit` and the documentation source from `docs/SOURCE`. Check disk and service state live rather than pinning an old PID or version number.
