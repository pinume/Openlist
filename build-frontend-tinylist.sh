#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
web_root="$repo_root/web"

command -v grep >/dev/null
command -v corepack >/dev/null

(
  cd "$web_root"
  CI=true corepack pnpm install --frozen-lockfile
  CI=true corepack pnpm build
  if [[ -d dist/static/monaco-editor/vs ]]; then
    find dist/static/monaco-editor/vs -type f \
      -name 'nls.messages.*.js.js' \
      ! -name 'nls.messages.js.js' \
      ! -name 'nls.messages.zh-cn.js.js' \
      -delete
  fi
  if LC_ALL=C grep -Rqi -- "aria2" dist ||
    LC_ALL=C grep -Eqi -- \
      "offline_download|qbittorrent|transmission" dist/assets/store-*.js ||
    LC_ALL=C grep -Eqi -- \
      "lightgallery|monaco-editor|pdf\\.js|artplayer|aplayer|docx-preview" \
      dist/assets/*.js ||
    LC_ALL=C grep -Eqi -- \
      "batch_download|playlist_download|copy_link|#EXTM3U" \
      dist/assets/*.js ||
    LC_ALL=C grep -Eqi -- \
      "guest-tips|ftp_read|ftp_manage|ssh_keys|s3_buckets|use_guest" \
      dist/assets/*.js; then
    echo "Removed feature code was found in the frontend build." >&2
    LC_ALL=C grep -Ril -- "aria2" dist >&2 || true
    LC_ALL=C grep -Eil -- \
      "offline_download|qbittorrent|transmission" dist/assets/store-*.js \
      >&2 || true
    LC_ALL=C grep -Eil -- \
      "lightgallery|monaco-editor|pdf\\.js|artplayer|aplayer|docx-preview" \
      dist/assets/*.js >&2 || true
    LC_ALL=C grep -Eil -- \
      "batch_download|playlist_download|copy_link|#EXTM3U" \
      dist/assets/*.js \
      >&2 || true
    LC_ALL=C grep -Eil -- \
      "guest-tips|ftp_read|ftp_manage|ssh_keys|s3_buckets|use_guest" \
      dist/assets/*.js \
      >&2 || true
    exit 1
  fi
  package_download_chunk="$(
    find dist/assets -maxdepth 1 -type f -name 'PackageDownload-*.js' -print -quit
  )"
  if [[ -z "$package_download_chunk" ]] ||
    ! LC_ALL=C grep -q -- '\.zip' "$package_download_chunk"; then
    echo "Folder package download was not found in the frontend build." >&2
    exit 1
  fi
)

mkdir -p "$repo_root/public/dist"
find "$repo_root/public/dist" -mindepth 1 ! -name README.md -delete
cp -a "$web_root/dist/." "$repo_root/public/dist/"

echo "Built TinyList frontend into $repo_root/public/dist"
