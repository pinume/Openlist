#!/usr/bin/env bash

set -euo pipefail

host_arch="$(go env GOHOSTARCH)"
output="${1:-dist/openlist-linux-${host_arch}}"

if [[ ! -f public/dist/index.html ]]; then
  echo "public/dist/index.html is missing." >&2
  echo "Build the private OpenList frontend and place its output in public/dist first." >&2
  exit 1
fi

go test ./server/middlewares ./drivers ./drivers/local ./drivers/dropbox ./drivers/s3

mkdir -p "$(dirname "$output")"
CGO_ENABLED=0 GOOS=linux GOARCH="$host_arch" \
  go build -trimpath -tags=jsoniter -ldflags="-s -w" -o "$output" .

echo "Built $output"
