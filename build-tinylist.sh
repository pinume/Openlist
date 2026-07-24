#!/usr/bin/env bash

set -euo pipefail

host_os="$(go env GOHOSTOS)"
if [[ "$host_os" != "linux" ]]; then
  echo "TinyList can only be built on Linux." >&2
  exit 1
fi

target_arch="${OPENLIST_GOARCH:-$(go env GOHOSTARCH)}"
output="${1:-dist/tinylist-linux-${target_arch}}"
built_at="${OPENLIST_BUILT_AT:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
git_author="${OPENLIST_GIT_AUTHOR:-$(git log -1 --format='%an <%ae>')}"
git_commit="${OPENLIST_GIT_COMMIT:-$(git rev-parse --short HEAD)}"
version="${OPENLIST_VERSION:-$(git describe --tags --exact-match 2>/dev/null || printf 'dev')}"
web_version="${OPENLIST_WEB_VERSION:-v4.2.4}"
ldflags="\
-s -w \
-X 'github.com/OpenListTeam/OpenList/v4/internal/conf.BuiltAt=$built_at' \
-X 'github.com/OpenListTeam/OpenList/v4/internal/conf.GitAuthor=$git_author' \
-X 'github.com/OpenListTeam/OpenList/v4/internal/conf.GitCommit=$git_commit' \
-X 'github.com/OpenListTeam/OpenList/v4/internal/conf.Version=$version' \
-X 'github.com/OpenListTeam/OpenList/v4/internal/conf.WebVersion=$web_version' \
"

if [[ ! -f public/dist/index.html ]]; then
  echo "public/dist/index.html is missing." >&2
  echo "Build the TinyList frontend and place its output in public/dist first." >&2
  exit 1
fi
if LC_ALL=C grep -Eqi -- \
  "lightgallery|monaco-editor|pdf\\.js|artplayer|aplayer|docx-preview" \
  public/dist/assets/*.js; then
  echo "public/dist still contains removed file preview code." >&2
  echo "Run ./build-frontend-tinylist.sh before building TinyList." >&2
  exit 1
fi
if [[ -d internal/fuse ]] ||
  grep -Eq -- "github.com/(winfsp/cgofuse|rclone/rclone)" go.mod; then
  echo "Removed FUSE/Crypt support is still present." >&2
  exit 1
fi

go test ./server/middlewares ./drivers ./drivers/local ./drivers/dropbox

mkdir -p "$(dirname "$output")"
CGO_ENABLED=0 GOOS=linux GOARCH="$target_arch" \
  go build -trimpath -tags=jsoniter -ldflags="$ldflags" -o "$output" .

echo "Built $output"
