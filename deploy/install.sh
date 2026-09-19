#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 || $# -ne 1 ]]; then
  echo 'Usage: sudo bash deploy/install.sh <linux-binary>' >&2; exit 64
fi
binary=$(realpath "$1")
scripts=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if [[ -e /opt/recipebox || -e /etc/systemd/system/recipebox.service ]] || id recipebox >/dev/null 2>&1; then
  echo 'Existing installation or identity found; use the documented upgrade procedure.' >&2; exit 1
fi
if ss -H -ltn 'sport = :18083' | grep -q .; then echo 'Port 18083 is in use.' >&2; exit 1; fi
command -v vips >/dev/null
command -v vipsheader >/dev/null
useradd --system --home-dir /opt/recipebox --shell /usr/sbin/nologin recipebox
install -d -m 0755 /opt/recipebox /opt/recipebox/bin /opt/recipebox/config /opt/recipebox/docs /opt/recipebox/releases
install -d -m 0700 -o recipebox -g recipebox /opt/recipebox/data /opt/recipebox/backups
install -m 0755 "$binary" /opt/recipebox/bin/recipebox
install -m 0644 "$scripts/nginx-location.conf" "$scripts/recipebox.service" "$scripts/recipebox-backup.service" "$scripts/recipebox-backup.timer" /opt/recipebox/config/
runuser -u recipebox -- /opt/recipebox/bin/recipebox init --data /opt/recipebox/data
systemctl link /opt/recipebox/config/recipebox.service /opt/recipebox/config/recipebox-backup.service /opt/recipebox/config/recipebox-backup.timer
systemctl daemon-reload
systemctl enable --now recipebox.service recipebox-backup.timer
curl --fail --silent --retry 10 --retry-delay 1 --retry-connrefused http://127.0.0.1:18083/healthz
systemctl start recipebox-backup.service
echo 'RecipeBox installed. Add its Nginx location after local health verification.'
