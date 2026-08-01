# TinyList

这是一个面向私有多用户场景的 TinyList 私有文件服务器，基于
[OpenList](https://github.com/OpenListTeam/OpenList) 精简。

## 功能边界

- 文件列表、文件详情、下载和 WebDAV 均要求认证。
- 浏览器直接下载使用登录后签发、固定五分钟有效的路径凭证；不提供永久下载或分享链接。
- 不包含游客角色、内置 `guest` 账号或匿名 API 访问。
- 仅包含 Local 和 Dropbox 两种存储驱动。
- 不提供 S3 存储驱动、S3 兼容 API 或独立 S3 监听服务。
- Dropbox 下载强制经过 TinyList 代理，使私有下载路由能够检查登录状态和用户根目录。
- 不包含离线下载和 Torrent 专用集成。
- 不包含需要 CGO/libfuse 的 FUSE 本地挂载能力。
- 管理端不包含分享、备份与还原、关于和文档入口。
- 不提供图片、音视频、PDF、Markdown、Office、压缩包等文件预览；打开文件会直接下载。
- 不生成或展示文件缩略图。
- 支持在浏览器中流式打包下载整个文件夹，不在服务端生成完整 ZIP 临时文件。
- 不提供上游压缩包预览使用的 `/ad`、`/ae`、`/ap` 下载接口或 Archive 签名。

静态资源、登录接口、SSO 回调、登录页需要的公开站点设置以及健康检查仍允许
匿名访问，以保证客户端能够展示登录页并完成认证。用户模型只保留管理员和普通
用户；启动时会清理旧数据库中角色编号为 `1` 的历史游客记录。

TinyList 不创建游客账号。已有数据库中的旧游客账号会在服务启动时删除；如需保留相关历史数据，请在升级前备份 SQLite 数据库。

新建或修改的密码使用 Argon2id 存储。旧版本的 SHA-256 密码记录仍可登录，并会
在下一次网页或 WebDAV 密码登录成功后自动升级，不要求用户集中重置密码。

## Local 存储边界

Local 驱动通过 Go `os.Root` 访问存储目录。相对符号链接可以指向存储根内的对象；
指向根外或使用绝对目标的符号链接不能用于读取、遍历或写入。回收站也必须位于
存储根内，配置为根外路径时存储初始化会失败。

升级已有部署前，应检查每个 Local 挂载的 `recycle_bin_path`，先把位于存储根外的
回收站迁移到根内并更新配置，否则该挂载会初始化失败。根内回收站仍属于挂载内容：
隐藏配置只影响目录列表，知道完整路径且具备读取权限的用户仍可按路径访问其中对象。
应把回收站放在受 Meta 读权限保护的隐藏目录中，而不应只依赖点号名称隐藏。

`os.Root` 限制的是路径解析，不是完整的操作系统沙箱：它不会阻止存储根内已有的
bind mount、特殊的 `/proc` 文件或设备文件跨越底层文件系统边界。生产环境仍应
保留 `deploy/tinylist.service` 中的 systemd 文件系统保护，并避免把设备或不可信
挂载点放进 Local 根目录。

上传文件不提供 per-storage 权限配置。新文件权限由进程 umask 控制；默认 systemd
unit 使用 `UMask=0027`。覆盖已有文件时沿用原文件权限，包括权限位为 `0000` 的文件。

启用目录大小统计后，隐藏文件同样计入大小。列表刷新只重算当前目录和父目录链；
缓存更新失败不会把已经成功的文件操作改报为失败，而是标记缓存失效，并在下次
刷新时重建。目录列表为避免逐项打开文件不返回精确创建时间；单对象详情仍会读取
普通文件和目录的创建时间。

## 用户隔离

沿用上游 OpenList 的用户模型：

- `role` 区分管理员和普通用户。
- `base_path` 将用户限制在指定目录树中。
- `permission` 控制上传、重命名、移动、复制、删除、WebDAV 和压缩包操作。
- Meta 的读写用户列表可以进一步限制具体目录。

管理员集中配置 Local、Dropbox 挂载，再通过管理 API 或管理界面设置用户的 `base_path` 和权限。

## 构建

构建过程完全离线，不会访问 Go Proxy、npm、GitHub、Crowdin 或其他上游服务。
构建机需要预装 `go.mod` 指定的 Go 工具链以及 Node.js 22.13 或更高版本；
工具链本身不在仓库内，构建脚本不会尝试在线安装或升级工具链。

前端源码已固化提交在 `web/`（导入来源、上游基线提交、许可证见
`web/UPSTREAM.md`），不再在构建期联网拉取上游仓库。

```bash
./build-frontend-tinylist.sh
```

仓库在 `third_party/pnpm/` 中包含固定版本的 pnpm 和
`web/pnpm-lock.yaml` 所需的完整依赖存储。脚本校验依赖存储的 SHA-256，
在临时目录中解包，并使用 `--offline --frozen-lockfile` 安装；不会使用
Corepack、用户级 pnpm 缓存或包注册表。随后脚本校验离线下载、文件预览等
已裁剪功能确实不存在、文件夹 ZIP 打包下载确实存在，并把产物放入
`public/dist`。升级前端到新的上游提交时，使用
`scripts/sync-frontend.sh <新 commit>` 生成差异，人工审查后再修改 `web/`，
具体流程见 `web/UPSTREAM.md`。

随后构建原生 Linux 二进制：

```bash
./build-tinylist.sh
```

Go 依赖已固化在 `vendor/`。构建脚本设置 `GOTOOLCHAIN=local`、
`GOPROXY=off` 和 `GOSUMDB=off`，并强制使用 `-mod=vendor`；缺少本地工具链
或 vendored 依赖时会直接失败。脚本只能在 Linux 上运行，并生成当前宿主
架构的 Linux 二进制。在 AArch64 主机上，输出为
`dist/tinylist-linux-arm64`。

修改 `go.mod`/`go.sum` 或 `web/package.json`/`web/pnpm-lock.yaml` 后，需要在
允许联网的维护环境中同步更新 `vendor/` 或 `third_party/pnpm/store-v11.tar.gz`；
普通构建环境不需要也不允许通过构建脚本更新依赖。

## 变更验证

合并或发布精简相关改动前，应在具备 `go.mod` 指定 Go 工具链的环境中执行：

```bash
gofmt -w $(git diff --name-only --diff-filter=ACM -- '*.go')
go mod tidy
go mod vendor
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test -mod=vendor ./...
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test -mod=vendor -race ./drivers/local ./server/webdav
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test -mod=vendor -race ./internal/sign/... ./pkg/sign/... ./server/handles/... ./server/middlewares/...
```

还需完成以下回归检查：

- 使用管理员和普通用户验证登录、退出、Token 重置及管理员 API 权限。
- 验证用户 `base_path`、Meta 密码、读写用户列表和隐藏文件权限仍能限制文件列表、详情、搜索及压缩包列表。
- 请求 `/api/fs/list` 时省略 `per_page` 并分别设置 `page=3`、`page=5`，确认返回空页而不是 HTTP 500。
- 验证 Local、Dropbox 上传、下载、复制、移动、删除和文件夹流式 ZIP 下载。
- 在启用目录大小统计的 Local 挂载上依次执行刷新、上传、删除、目录重命名和再次
  刷新，使用包含隐藏文件的独立 `filepath.WalkDir` 统计结果核对根目录及父目录大小。
- 验证 Local 根内相对符号链接可访问，根外和绝对目标符号链接不能越过存储根。
- 验证 WebDAV COPY/MOVE 的 `Overwrite: F` 返回 412，覆盖成功返回 204，新建目标
  返回 201，并且目标名称与源名称不同时不会碰撞同目录的无关文件。
- 验证 WebDAV、FTP、SFTP 在缺少或无效用户上下文时拒绝访问，正常账号仍可按权限读写。
- 重置管理员 Token 的同时并发访问签名下载接口，并使用 `go test -race` 确认没有签名实例数据竞争。
- 确认浏览器下载凭证只绑定一个路径且约五分钟后失效，升级时会删除旧的 `link_expiration` 设置。
- 使用旧 SHA-256 密码记录登录，确认成功后数据库记录自动升级为 Argon2id，错误密码仍被拒绝。
- 确认 `/s3` 及独立 S3 端口不可用，生成的 `config.json` 不再包含 `s3` 配置段。
- 确认压缩包元数据响应不再返回指向 `/ad`、`/ae` 的 `raw_url` 或 Archive `sign`。
- 使用已有数据目录升级时，确认旧游客账号被清理，其他用户、存储和权限数据保持不变。

如果当前环境无法执行上述命令，提交或 Pull Request 必须明确写明未运行的项目，不得将静态检查描述为编译或测试通过。

## Linux 安装

创建服务账号和目录：

```bash
sudo useradd --system --home /opt/tinylist --shell /usr/sbin/nologin tinylist
sudo install -d -o tinylist -g tinylist -m 0750 /opt/tinylist/data
sudo install -o root -g root -m 0755 tinylist /opt/tinylist/tinylist
sudo install -o root -g root -m 0644 deploy/tinylist.service /etc/systemd/system/tinylist.service
```

首次启动会生成管理员密码，并且只输出到服务控制台。也可以提前指定：

```bash
sudo systemctl edit tinylist
```

```ini
[Service]
Environment=OPENLIST_ADMIN_PASSWORD=请替换为高强度密码
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now tinylist
sudo journalctl -u tinylist -f
```

首次启动会创建 `/opt/tinylist/data/config.json`。默认 HTTP 监听地址为
`0.0.0.0:5244`。程序不提供内置 HTTPS、FTP、SFTP、S3 或 MCP 服务。建议使用
Caddy 或 Nginx 提供 TLS，并通过主机防火墙限制外部直接访问 5244 端口。
