#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 || $# -ne 1 ]]; then echo 'Usage: sudo bash deploy/setup-ci.sh <public-key.pub>' >&2; exit 64; fi
public_key=$(realpath "$1")
scripts=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
ssh-keygen -l -f "$public_key" >/dev/null
if [[ $(wc -l < "$public_key") -ne 1 ]] || ! grep -q '^ssh-ed25519 ' "$public_key"; then echo 'Expected one Ed25519 public key.' >&2; exit 64; fi
test -x /opt/recipebox/bin/recipebox
if id recipebox-deploy >/dev/null 2>&1 || [[ -e /etc/sudoers.d/recipebox-deploy ]]; then echo 'CI identity already exists.' >&2; exit 1; fi
useradd --system --home-dir /opt/recipebox/deploy-user --shell /bin/bash recipebox-deploy
install -d -m 0755 /opt/recipebox/deploy-user /opt/recipebox/deploy-user/.ssh
install -m 0755 "$scripts/deploy-ssh.sh" "$scripts/deploy-release.sh" /opt/recipebox/bin/
printf 'restrict,command="/opt/recipebox/bin/deploy-ssh.sh" %s\n' "$(< "$public_key")" > /opt/recipebox/deploy-user/.ssh/authorized_keys
chmod 0644 /opt/recipebox/deploy-user/.ssh/authorized_keys
printf 'recipebox-deploy ALL=(root) NOPASSWD: /opt/recipebox/bin/deploy-release.sh\n' > /etc/sudoers.d/recipebox-deploy
chmod 0440 /etc/sudoers.d/recipebox-deploy
visudo -cf /etc/sudoers.d/recipebox-deploy
