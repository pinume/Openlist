import { readFile, readdir, unlink, writeFile } from "node:fs/promises"
import { join } from "node:path"

const langDir = process.argv[2]
if (!langDir) {
  throw new Error("usage: node scripts/prune-frontend-i18n.mjs <lang-directory>")
}

const readJSON = async (name) =>
  JSON.parse(await readFile(join(langDir, name), "utf8"))

const writeJSON = async (name, value) =>
  writeFile(join(langDir, name), `${JSON.stringify(value, null, 2)}\n`)

const deleteKeys = (object, keys) => {
  for (const key of keys) {
    delete object[key]
  }
}

const keepKeys = (object, keys) =>
  Object.fromEntries(
    keys.filter((key) => key in object).map((key) => [key, object[key]]),
  )

const driverNames = ["Local", "Dropbox", "S3"]
const drivers = await readJSON("drivers.json")
const prunedDrivers = keepKeys(drivers, driverNames)
prunedDrivers.config = keepKeys(drivers.config, driverNames)
prunedDrivers.drivers = keepKeys(drivers.drivers, driverNames)
await writeJSON("drivers.json", prunedDrivers)

const home = await readJSON("home.json")
deleteKeys(home.toolbar, [
  "toggle_theme",
  "share",
  "offline_download",
  "offline_download-tips",
  "offline_download_torrent",
  "offline_download_enhanced",
  "delete_policy",
  "send_aria2",
  "aria2_not_set",
  "send_aria2_success",
])
deleteKeys(home.local_settings, [
  "aria2_rpc_url",
  "aria2_rpc_secret",
  "aria2_dir",
])
home.footer.powered_by = "由 TinyList 驱动"
await writeJSON("home.json", home)

const login = await readJSON("login.json")
login.title = "登录到 TinyList"
await writeJSON("login.json", login)

const manage = await readJSON("manage.json")
manage.title = "TinyList 管理"
manage.sidemenu = keepKeys(manage.sidemenu, [
  "profile",
  "users",
  "storages",
  "home",
])
await writeJSON("manage.json", manage)

const settings = await readJSON("settings.json")
for (const key of Object.keys(settings)) {
  if (
    /^(?:aria2|offline_download|qbittorrent|transmission|share_)/i.test(key) ||
    /^(?:115|123|pikpak|thunder).*temp_dir$/i.test(key) ||
    key === "123_open_callback_url"
  ) {
    delete settings[key]
  }
}
await writeJSON("settings.json", settings)

const settingsOther = await readJSON("settings_other.json")
await writeJSON(
  "settings_other.json",
  keepKeys(settingsOther, [
    "copy_token",
    "reset_token",
    "reset_token_success",
    "unknown_type",
  ]),
)

const tasks = await readJSON("tasks.json")
deleteKeys(tasks, ["offline_download", "offline_download_transfer"])
delete tasks.attr.offline_download
await writeJSON("tasks.json", tasks)

const users = await readJSON("users.json")
deleteKeys(users.permissions, [
  "offline_download",
  "share",
  "customize_share_id",
])
await writeJSON("users.json", users)

for (const name of ["br.json", "indexes.json", "shares.json"]) {
  await unlink(join(langDir, name))
}

const entryPath = join(langDir, "entry.ts")
let entry = await readFile(entryPath, "utf8")
entry = entry
  .replace(/^import (?:br|indexes|shares) from .*\n/gm, "")
  .replace(/^  (?:br|indexes|shares),\n/gm, "")
await writeFile(entryPath, entry)

const forbidden = /aria2|offline_download|qbittorrent|transmission/i
for (const name of await readdir(langDir)) {
  if (!name.endsWith(".json") && name !== "entry.ts") {
    continue
  }
  const contents = await readFile(join(langDir, name), "utf8")
  if (forbidden.test(contents)) {
    throw new Error(`removed feature translation remains in ${name}`)
  }
}
