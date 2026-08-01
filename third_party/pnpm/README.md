# Vendored pnpm dependencies

TinyList vendors pnpm 11.10.0 and a complete pnpm v11 content-addressable
store so the frontend can be built without network access.

- `11.10.0/` is the unmodified `pnpm@11.10.0` package distributed under the
  MIT license recorded in `11.10.0/LICENSE`.
- `store-v11.tar.gz` contains every package referenced by
  `web/pnpm-lock.yaml`, including optional packages for supported platforms.
- The archive SHA-256 is
  `1d27c1ef5b3dd508020bb32a19a0752754b8af5f08c9b64ea1fe1327116388c8`.

The normal build verifies the archive, extracts it into a temporary
directory, and runs pnpm with `--offline --frozen-lockfile`. It never uses
Corepack or a user-level pnpm store.

When `web/package.json` or `web/pnpm-lock.yaml` changes, regenerate this
archive in a connected maintenance environment, update its SHA-256 here and
in `build-frontend-tinylist.sh`, then verify the build with empty user caches
and blocked network access.
