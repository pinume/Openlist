#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
frontend_repo="${OPENLIST_FRONTEND_REPO:-https://github.com/OpenListTeam/OpenList-Frontend.git}"
frontend_commit="${OPENLIST_FRONTEND_COMMIT:-0d149d1ac40087556a36efecf11a51c012882e57}"
i18n_url="https://github.com/OpenListTeam/OpenList-Frontend/releases/download/v4.2.4/i18n.tar.gz"
i18n_sha256="f969170b947a185baef431dc6dabcfd90ed3826b535438661fdf84d6d076a38b"
frontend_patch_dir="$repo_root/patches/frontend"
frontend_patches=(
  personal.patch
  chinese-ui.patch
  size.patch
  aria2-removal.patch
  tinylist.patch
  preview-removal.patch
  preview-dependencies.patch
  preview-build.patch
  folder-download.patch
  manual-download.patch
  header-logo.patch
  download-options-removal.patch
  service-removal.patch
)
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
for patch in "${frontend_patches[@]}"; do
  git -C "$work_root/frontend" apply "$frontend_patch_dir/$patch"
done

find "$work_root/frontend/src/pages/home/previews" -depth -delete
find "$work_root/frontend/src/components/artplayer-plugin-ass" -depth -delete
find "$work_root/frontend/public/images" -maxdepth 1 -type f \
  \( -name 'figplayer.webp' -o -name 'iina.webp' -o -name 'infuse.webp' \
  -o -name 'mpv.webp' -o -name 'mxplayer*.webp' -o -name 'nplayer.webp' \
  -o -name 'omniplayer.webp' -o -name 'potplayer.webp' \
  -o -name 'vlc.webp' \) -delete
rm -f \
  "$work_root/frontend/src/components/EncodingSelect.tsx" \
  "$work_root/frontend/src/components/Markdown.tsx" \
  "$work_root/frontend/src/components/MonacoEditor.tsx" \
  "$work_root/frontend/src/components/markdown.css" \
  "$work_root/frontend/src/pages/home/Readme.tsx" \
  "$work_root/frontend/src/pages/home/file/open-with.tsx" \
  "$work_root/frontend/src/pages/home/folder/ImageItem.tsx" \
  "$work_root/frontend/src/pages/home/folder/Images.tsx"
rm -f \
  "$work_root/frontend/src/pages/manage/settings/S3.tsx" \
  "$work_root/frontend/src/pages/manage/settings/S3BucketItem.tsx" \
  "$work_root/frontend/src/pages/manage/settings/S3Buckets.tsx" \
  "$work_root/frontend/src/pages/manage/users/PublicKey.tsx" \
  "$work_root/frontend/src/pages/manage/users/PublicKeys.tsx" \
  "$work_root/frontend/src/types/sshkey.ts"

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
cp -a "$work_root/frontend/dist/." "$repo_root/public/dist/"

echo "Built TinyList frontend into $repo_root/public/dist"
