#!/usr/bin/env bash
set -euo pipefail
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
umask 077

# The deployment identity may publish only this application's bounded declaration.
# All validation and atomic publication logic belongs to ServerPortal/server-operations.
if [[ $EUID -eq 0 && $# -eq 3 && ( $1 == portal || $1 == portal-check ) && $2 =~ ^[0-9a-f]{40}$ && $3 =~ ^[0-9a-f]{64}$ ]]; then
  portal_action=register
  if [[ $1 == portal-check ]]; then portal_action=check-registration; fi
  exec timeout 45 /opt/serverportal/bin/portal "$portal_action" --app recipebox --commit "$2" --sha256 "$3"
fi

if [[ $EUID -ne 0 || $# -lt 2 || $# -gt 3 || ! $1 =~ ^[0-9a-f]{40}$ || ! $2 =~ ^[0-9a-f]{64}$ ]]; then echo 'Usage: deploy-release.sh <commit> <sha256> < binary' >&2; exit 64; fi
commit=$1
expected=$2
plan_token=${3:-}
if [[ -n $plan_token && ( ! $plan_token =~ ^[A-Za-z0-9_-]+$ || ${#plan_token} -gt 4096 ) ]]; then
  echo 'Invalid database protection plan.' >&2; exit 64
fi
app=/opt/recipebox
nas=false
upgrade=/opt/server-context/scripts/upgrade-state.py
if [[ -r /etc/server-maintenance-host && $(< /etc/server-maintenance-host) == nas ]]; then
  nas=true
  test -f "$upgrade"
  exec 8>/run/lock/server-maintenance.lock
  flock -s -n 8 || { printf 'A full backup or maintenance operation is running.\n' >&2; exit 75; }
fi
if [[ -n $plan_token && $nas != true ]]; then echo 'Database plans require the NAS release flow.' >&2; exit 64; fi
exec 9>/run/lock/recipebox-deploy.lock
flock -n 9 || { echo 'Another release is running.' >&2; exit 75; }
test -f "$app/data/recipebox.db"
test -x "$app/bin/recipebox"
release=$(mktemp -d "$app/releases/$commit.XXXXXX")
chmod 0755 "$release"
stopped=false
replaced=false
healthy() {
  systemctl is-active --quiet recipebox &&
    curl --fail --silent --max-time 3 http://127.0.0.1:18083/healthz | grep -q '"status":"ok"' &&
    curl --fail --silent --max-time 3 http://127.0.0.1/recipebox/new | grep -q '/recipebox/assets/'
}
wait_healthy() { for ((attempt=0;attempt<20;attempt++)); do if healthy; then return 0; fi; sleep 1; done; return 1; }
finish() {
  result=$?
  trap - EXIT HUP INT TERM
  recovery_status=not-needed
  if [[ $result -ne 0 && $stopped == true ]]; then
    recovery_status=failed
    systemctl stop recipebox || true
    if [[ $replaced == true ]]; then install -m 0755 "$release/previous" "$app/bin/recipebox.rollback"; mv -f "$app/bin/recipebox.rollback" "$app/bin/recipebox"; fi
    if systemctl start recipebox && wait_healthy; then recovery_status=healthy; echo 'Previous program is healthy; database was not rolled back.' >&2;
    else echo 'ROLLBACK FAILED: inspect journalctl -u recipebox.' >&2; fi
  fi
  printf '%s\n' "$recovery_status" > "$release/recovery"
  if [[ $result -eq 0 ]]; then echo success > "$release/result"; else echo failed > "$release/result"; fi
  exit "$result"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
timeout 90 head -c 67108865 > "$release/recipebox"
size=$(stat -c %s "$release/recipebox")
if ((size==0||size>67108864)); then echo 'Binary size must be 1 byte to 64 MiB.' >&2; exit 65; fi
actual=$(sha256sum "$release/recipebox")
if [[ ${actual%% *} != "$expected" ]]; then echo 'Checksum mismatch.' >&2; exit 65; fi
chmod 0755 "$release/recipebox"
cp "$app/bin/recipebox" "$release/previous"
chmod 0755 "$release/previous"
printf 'commit=%s\nsha256=%s\n' "$commit" "$expected" > "$release/metadata"
backup="$app/backups/before-deploy-$(basename "$release")"
stopped=true
systemctl stop recipebox
if [[ $nas == true ]]; then
  candidate_data=$(python3 "$upgrade" begin --app recipebox --release "$(basename "$release")" --plan-token "$plan_token")
  backup="/var/lib/server-upgrades/recipebox/$(basename "$release")"
  runuser -u recipebox -- timeout 30 "$release/recipebox" check --data "$candidate_data"
else
  runuser -u recipebox -- timeout 180 "$app/bin/recipebox" backup --data "$app/data" --out "$backup"
fi
# Uploaded executable always runs as the application identity.
if [[ $nas != true ]]; then
  runuser -u recipebox -- timeout 30 "$release/recipebox" check --data "$app/data"
fi
install -m 0755 "$release/recipebox" "$app/bin/recipebox.next"
replaced=true
mv -f "$app/bin/recipebox.next" "$app/bin/recipebox"
systemctl start recipebox
wait_healthy
if [[ $nas == true && -d $backup ]]; then
  systemctl stop recipebox
  python3 "$upgrade" verify --app recipebox --release "$(basename "$release")"
  systemctl start recipebox
  wait_healthy
fi
printf '%s\n' "$commit" > "$app/current-commit"
chmod 0644 "$app/current-commit"
if [[ $nas == true ]]; then python3 "$upgrade" accept --app recipebox --release "$(basename "$release")"; fi
printf 'Deployed %s.\n' "$commit"
