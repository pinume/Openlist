#!/usr/bin/env bash

# Fetches a new OpenList-Frontend commit and prints its diff against the
# commit that web/ is currently based on (see web/UPSTREAM.md), so upstream
# updates can be reviewed and applied to web/ by hand instead of being
# pulled in automatically at build time.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
frontend_repo="${OPENLIST_FRONTEND_REPO:-https://github.com/OpenListTeam/OpenList-Frontend.git}"

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <new-upstream-commit>" >&2
  exit 1
fi
new_commit="$1"

base_commit="$(grep -m1 -oE '`[0-9a-f]{40}`' "$repo_root/web/UPSTREAM.md" | tr -d '`')"
if [[ -z "$base_commit" ]]; then
  echo "Could not find the current baseline commit in web/UPSTREAM.md." >&2
  exit 1
fi

command -v git >/dev/null

work_root="$(mktemp -d)"
cleanup() {
  rm -rf -- "$work_root"
}
trap cleanup EXIT

git clone --filter=blob:none --no-checkout "$frontend_repo" "$work_root/frontend"
git -C "$work_root/frontend" fetch --depth=1 origin "$base_commit"
git -C "$work_root/frontend" fetch --depth=1 origin "$new_commit"

echo "Diff from $base_commit to $new_commit:" >&2
git -C "$work_root/frontend" diff "$base_commit" "$new_commit"

cat >&2 <<EOF

This only shows the upstream diff. Review it, apply the relevant changes to
web/ by hand (keeping the TinyList customizations documented in
web/UPSTREAM.md), then update the baseline commit recorded there and commit
the sync as its own change.
EOF
