#!/usr/bin/env bash

set -euo pipefail

web_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$web_root/.." && pwd)"

if [[ $# -ne 0 ]]; then
  echo "web/build.sh no longer accepts upstream release or i18n options." >&2
  echo "Run $repo_root/build-frontend-tinylist.sh without arguments." >&2
  exit 2
fi

exec "$repo_root/build-frontend-tinylist.sh"
