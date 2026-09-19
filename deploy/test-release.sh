#!/usr/bin/env bash
# Run release failure paths with isolated files and simulated service commands.
# The production script keeps its fixed paths and root-only invocation.
set -euo pipefail
if [[ $(uname -s) != Linux ]]; then echo 'Release integration checks require Linux.' >&2; exit 1; fi
scripts=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
release_test=$(mktemp -d /tmp/recipebox-release-test.XXXXXX)
trap 'rm -rf -- "$release_test"' EXIT
export TEST_COMMANDS="$release_test/commands"
mkdir "$TEST_COMMANDS"
sed -e 's|^export PATH=.*|export PATH="$TEST_COMMANDS:/usr/bin:/bin"|' \
  -e 's/\$EUID -ne 0/0 -ne 0/' \
  -e 's|^app=/opt/recipebox$|app=$TEST_APP|' \
  -e 's|^exec 9>/run/lock/recipebox-deploy.lock$|exec 9>"$TEST_APP/deploy.lock"|' \
  "$scripts/deploy-release.sh" > "$release_test/deploy.sh"
cat > "$TEST_COMMANDS/systemctl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ $* == *recipebox* ]] || exit 90
printf '%s\n' "$*" >> "$TEST_APP/service.log"
case $1 in
  stop) echo stopped > "$TEST_APP/service-state" ;;
  start) echo active > "$TEST_APP/service-state" ;;
  is-active) [[ $(cat "$TEST_APP/service-state") == active ]] ;;
  *) exit 91 ;;
esac
SH
cat > "$TEST_COMMANDS/curl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if grep -q '^# unhealthy$' "$TEST_APP/bin/recipebox"; then exit 22; fi
printf '%s\n' '{"status":"ok"}' '/recipebox/assets/test.js'
SH
cat > "$TEST_COMMANDS/runuser" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ $1 == -u && $2 == recipebox && $3 == -- ]] || exit 92
shift 3
exec "$@"
SH
printf '#!/usr/bin/env bash\nexit 0\n' > "$TEST_COMMANDS/sleep"
chmod 0755 "$TEST_COMMANDS/"*
cat > "$release_test/previous" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ $1 == backup ]]; then mkdir "$5"; printf backup > "$5/manifest.json"; fi
SH
cp "$release_test/previous" "$release_test/good"
printf '\n# good candidate\n' >> "$release_test/good"
printf '#!/usr/bin/env bash\n[[ $1 != check ]]\n' > "$release_test/bad-check"
cp "$release_test/good" "$release_test/unhealthy"
printf '\n# unhealthy\n' >> "$release_test/unhealthy"
commit=1111111111111111111111111111111111111111
old_commit=2222222222222222222222222222222222222222
for scenario in checksum bad-check unhealthy good; do
  export TEST_APP="$release_test/$scenario-app"
  mkdir -p "$TEST_APP/"{bin,data,backups,releases}
  cp "$release_test/previous" "$TEST_APP/bin/recipebox"
  chmod 0755 "$TEST_APP/bin/recipebox"
  printf 'synthetic database\n' > "$TEST_APP/data/recipebox.db"
  cp "$TEST_APP/data/recipebox.db" "$TEST_APP/db-before"
  echo active > "$TEST_APP/service-state"
  echo "$old_commit" > "$TEST_APP/current-commit"
  candidate="$release_test/$scenario"
  if [[ $scenario == checksum ]]; then candidate="$release_test/good"; fi
  digest=$(sha256sum "$candidate"); digest=${digest%% *}
  if [[ $scenario == checksum ]]; then digest=0000000000000000000000000000000000000000000000000000000000000000; fi
  result=0
  bash "$release_test/deploy.sh" "$commit" "$digest" < "$candidate" > "$TEST_APP/output" 2>&1 || result=$?
  if [[ $scenario == good ]]; then
    [[ $result == 0 ]] || { cat "$TEST_APP/output"; exit 1; }
    cmp "$candidate" "$TEST_APP/bin/recipebox"
    [[ $(cat "$TEST_APP/current-commit") == "$commit" ]]
    grep -qx success "$TEST_APP/releases/"*/result
  else
    [[ $result != 0 ]]
    cmp "$release_test/previous" "$TEST_APP/bin/recipebox"
    [[ $(cat "$TEST_APP/current-commit") == "$old_commit" ]]
    grep -qx failed "$TEST_APP/releases/"*/result
    if [[ $scenario == checksum ]]; then
      [[ ! -e $TEST_APP/service.log ]]
    else
      grep -q 'Previous program is healthy' "$TEST_APP/output"
    fi
  fi
  cmp "$TEST_APP/db-before" "$TEST_APP/data/recipebox.db"
  [[ $(cat "$TEST_APP/service-state") == active ]]
  printf 'PASS release %s\n' "$scenario"
done
