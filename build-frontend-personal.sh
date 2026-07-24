#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
frontend_repo="${OPENLIST_FRONTEND_REPO:-https://github.com/OpenListTeam/OpenList-Frontend.git}"
frontend_commit="${OPENLIST_FRONTEND_COMMIT:-0d149d1ac40087556a36efecf11a51c012882e57}"
i18n_url="https://github.com/OpenListTeam/OpenList-Frontend/releases/download/v4.2.4/i18n.tar.gz"
i18n_sha256="f969170b947a185baef431dc6dabcfd90ed3826b535438661fdf84d6d076a38b"
work_root="$(mktemp -d)"

cleanup() {
  rm -rf -- "$work_root"
}
trap cleanup EXIT

command -v git >/dev/null
command -v curl >/dev/null
command -v grep >/dev/null
command -v sha256sum >/dev/null
command -v tar >/dev/null
command -v node >/dev/null
command -v corepack >/dev/null

git clone --filter=blob:none --no-checkout "$frontend_repo" "$work_root/frontend"
git -C "$work_root/frontend" fetch --depth=1 origin "$frontend_commit"
git -C "$work_root/frontend" switch --detach "$frontend_commit"
git -C "$work_root/frontend" apply "$repo_root/frontend-personal.patch"
git -C "$work_root/frontend" apply "$repo_root/frontend-chinese-ui.patch"
git -C "$work_root/frontend" apply "$repo_root/frontend-size.patch"
git -C "$work_root/frontend" apply "$repo_root/frontend-aria2-removal.patch"

curl -fsSL "$i18n_url" -o "$work_root/i18n.tar.gz"
printf '%s  %s\n' "$i18n_sha256" "$work_root/i18n.tar.gz" | sha256sum --check -
tar -xzf "$work_root/i18n.tar.gz" -C "$work_root/frontend/src/lang" ./zh-CN
node "$repo_root/scripts/prune-frontend-i18n.mjs" \
  "$work_root/frontend/src/lang/zh-CN"
find "$work_root/frontend/src/lang/en" -depth -delete

(
  cd "$work_root/frontend"
  corepack pnpm install --frozen-lockfile
  corepack pnpm build
  find dist/static/monaco-editor/vs -type f \
    -name 'nls.messages.*.js.js' \
    ! -name 'nls.messages.js.js' \
    ! -name 'nls.messages.zh-cn.js.js' \
    -delete
  if LC_ALL=C grep -Rqi -- "aria2" dist ||
    LC_ALL=C grep -Eqi -- \
      "offline_download|qbittorrent|transmission" dist/assets/store-*.js; then
    echo "Removed feature code was found in the frontend build." >&2
    LC_ALL=C grep -Ril -- "aria2" dist >&2 || true
    LC_ALL=C grep -Eil -- \
      "offline_download|qbittorrent|transmission" dist/assets/store-*.js \
      >&2 || true
    exit 1
  fi
)

mkdir -p "$repo_root/public/dist"
find "$repo_root/public/dist" -mindepth 1 ! -name README.md -delete
cp -a "$work_root/frontend/dist/." "$repo_root/public/dist/"

echo "Built private frontend into $repo_root/public/dist"
