# OpenList Personal Server 维护说明

本文档记录本仓库从上游 OpenList 精简为私有 Linux 文件服务器的设计边界、
实施方式和后续升级流程。维护时应先阅读本文，再修改后端、前端补丁或发布流程。

## 项目定位

本项目面向个人和小团队，仅支持 Linux `arm64` 原生单二进制部署。它不是上游
OpenList 的通用发行版，也不保持容器和多操作系统兼容性。

核心目标：

- 所有文件访问均要求登录。
- 用户只能访问其 `base_path` 和权限允许的目录与操作。
- 仅保留 Local、Dropbox 和 S3 存储驱动。
- Dropbox 与 S3 下载经过服务端代理，以执行认证和目录权限检查。
- 使用简体中文、现代浏览器前端。
- 使用 systemd 管理服务，由 Caddy 或 Nginx 提供 HTTPS。

## Git 与前端基线

后端精简的 Git 历史基线：

- 精简前父提交：`84ecda35`
- 个人服务器精简提交：`075304fb`

前端构建固定使用：

- 仓库：`OpenListTeam/OpenList-Frontend`
- 提交：`0d149d1ac40087556a36efecf11a51c012882e57`
- 对应版本：`v4.2.4`
- 简体中文词典：前端 `v4.2.4` Release 中的 `i18n.tar.gz`
- 词典 SHA-256：
  `f969170b947a185baef431dc6dabcfd90ed3826b535438661fdf84d6d076a38b`

这些值集中在 `build-frontend-personal.sh` 中。更新上游前端时，必须同步更新提交、
词典版本、校验值和补丁，不能只替换其中一个。

## 保留能力

- 用户、登录、会话、2FA、SSO 和权限管理。
- 文件列表、详情、搜索、上传、下载、移动、复制、重命名和删除。
- 图片、音视频、PDF、Markdown、Office、文本和压缩包预览。
- WebDAV、FTP 和 SFTP。
- SQLite、MySQL 和 PostgreSQL。
- Local、Dropbox 和兼容 AWS S3 API 的对象存储。
- 管理存储、用户和服务器运行所需的后台任务。

通用任务管理代码不能仅因名称中包含 `task` 而删除。复制、移动、上传、解压等保留
功能仍依赖 `internal/task`、`internal/task_group` 和 `pkg/task`。

## 已删除能力

- 游客、匿名文件访问和公开站点模式。
- 分享记录、分享链接和永久公开链接。
- 离线下载及其 Aria2、qBittorrent、Transmission、迅雷等工具集成。
- Torrent 离线下载入口和相关后端 API。
- 配置备份与还原。
- Local、Dropbox、S3 之外的存储驱动。
- 管理端分享、备份还原、关于、文档、索引、任务等非必要导航入口。
- Dockerfile、Compose、容器入口脚本、镜像构建和镜像清理工作流。
- Windows、macOS、Android、FreeBSD 和 OpenWRT 构建发布流程。
- Windows 专用进程、文件系统和内存实现。
- 上游多语言 README 和前端语言切换。

协议兼容文本不等于平台支持。例如 Windows WebDAV 客户端兼容逻辑、APK MIME
类型和驱动使用的浏览器 User-Agent 可以保留。

## 后端精简约束

以下行为是安全边界，升级或合并上游时必须重新核对：

1. 文件列表、详情、下载、预览、压缩包访问和文件操作必须经过认证。
2. WebDAV、FTP 和 SFTP 必须执行用户基础路径与权限检查。
3. Dropbox 和 S3 不得绕过服务端生成匿名可访问的私有下载路径。
4. 登录页、静态资源、登录/SSO 回调、必要公开设置和健康检查可以匿名访问。
5. `drivers/all.go` 只能注册 Local、Dropbox 和 S3。
6. 不得重新引入 `internal/offline_download`、`pkg/aria2` 或对应路由与设置。

## 前端补丁流程

`build-frontend-personal.sh` 在临时目录中检出固定前端提交，并按以下顺序处理：

1. 应用 `frontend-personal.patch`：删除分享、离线下载和非必要管理功能。
2. 应用 `frontend-chinese-ui.patch`：收敛到简体中文界面。
3. 应用 `frontend-size.patch`：删除旧浏览器资源并控制产物大小。
4. 应用 `frontend-aria2-removal.patch`：删除 Aria2 本地设置和设置组。
5. 下载并校验固定版本的简体中文词典。
6. 运行 `scripts/prune-frontend-i18n.mjs`，只保留三个驱动的词典，并删除分享、
   备份、离线下载、Aria2、qBittorrent 和 Transmission 等已移除功能的词典。
7. 删除英文词典，执行 TypeScript 和生产构建。
8. 删除未使用的 Monaco Editor 语言资源。
9. 检查最终 `dist`；任何文件出现 `aria2`，或应用词典 bundle 出现
   `offline_download`、`qbittorrent`、`transmission` 时构建失败。

`frontend-personal.patch` 中仍可看到 Aria2 字样，因为统一 diff 必须包含被删除的
上游原文才能可靠应用。以 `-` 开头的内容是删除指令，不代表最终产物仍包含该功能。
不要为了让文本搜索结果为零而删除这些负向差异，否则会把上游功能重新带回。

更新前端时建议：

```bash
./build-frontend-personal.sh
grep -REni --binary-files=text \
  "aria2" public/dist
grep -Ei "offline_download|qbittorrent|transmission" \
  public/dist/assets/store-*.js
```

两条检查命令都应无输出。Monaco、字幕和 Torrent 预览等第三方资源可能在内部协议
代码中使用 `transmission` 等普通英文单词，因此非 Aria2 关键词仅检查应用词典
bundle。随后检查登录、文件列表、上传、下载、预览和管理存储页面。

## Linux 构建与部署

构建：

```bash
./build-frontend-personal.sh
./build-personal.sh
```

`build-personal.sh` 会拒绝非 Linux 宿主，默认生成当前宿主架构的 Linux 二进制。
CI 和 Release 工作流通过 `OPENLIST_GOARCH=arm64` 生成
`dist/openlist-linux-arm64`。

部署：

```bash
sudo install -o root -g root -m 0755 dist/openlist-linux-arm64 /opt/openlist/openlist
sudo install -o root -g root -m 0644 deploy/openlist.service /etc/systemd/system/openlist.service
sudo systemctl daemon-reload
sudo systemctl enable --now openlist
```

数据目录为 `/opt/openlist/data`。生产环境应限制直接访问 `5244` 端口，并通过
Caddy 或 Nginx 提供 TLS。

## 验证清单

每次维护至少执行：

```bash
bash -n build-frontend-personal.sh build-personal.sh
git diff --check
./build-personal.sh
```

`build-personal.sh` 当前执行以下核心测试：

```bash
go test ./server/middlewares ./drivers ./drivers/local ./drivers/dropbox ./drivers/s3
```

还应确认：

- 产物为静态链接的 Linux arm64 ELF。
- `openlist version` 能正常运行。
- Docker、Compose 和非 Linux 发布入口没有重新出现。
- 后端和最终前端产物中没有 Aria2 API、设置或界面入口。
- 未认证用户无法读取文件信息或内容。
- 普通用户无法越过 `base_path`。
- Local、Dropbox 和 S3 仍可完成上传与下载。

`go test ./...` 在缺少 FUSE/CGO 开发环境时可能无法编译 `internal/fuse`。某些受控
执行环境还会注入 HTTP 安全传输层，导致 `internal/net` 的代理类型断言失败。这些
情况应单独记录，不能宣称全量测试通过，也不应在没有复现业务问题时修改无关代码。

## 保留的兼容骨架

以下关键词可能与已删除功能同名，但对应代码仍有明确用途：

- 禁用的 Guest 角色仍用于数据库角色编号、旧数据兼容和认证拒绝逻辑；保留它不代表
  允许游客读取文件。
- `MaxBackups` 控制日志文件轮转，不是配置备份与还原功能。
- WebDAV 的 `shared` 表示锁协议范围，不是公开分享功能。
- `github.com/bodgit/windows` 和 `github.com/go-darwin/apfs` 是归档或 rclone
  依赖链中的间接模块，不是本项目的 Windows/macOS 发布支持。
- Windows User-Agent、APK MIME 类型和 Windows WebDAV 客户端说明用于外部客户端
  兼容，不应按文件名关键词机械删除。

## 变更记录

### 2026-07-24：Linux 专用部署与残留清理

- 删除 `Dockerfile`、`Dockerfile.ci`、`docker-compose.yml`、`entrypoint.sh`
  及容器构建、测试、发布、镜像清理工作流。
- 删除旧的 `build.sh`、Windows Zig wrapper、OpenWRT 触发工作流和 Beta 多平台
  发布工作流。
- 删除 Windows 专用停止命令、Local 文件系统和内存实现；Linux 实现改用明确的
  `linux` 构建标签。
- 将 GitHub Build 和 Release 工作流改为只构建、打包 Linux arm64 原生二进制。
- `build-personal.sh` 增加 Linux 宿主检查，并允许 CI 通过
  `OPENLIST_GOARCH=arm64` 指定目标架构。
- 删除上游多语言 README，清理安全文档中的容器部署建议，并更新 Linux/systemd
  部署说明。
- 确认个人版提交已经删除 `internal/offline_download`、`pkg/aria2`、离线下载
  路由/API 和相应 Go 依赖。
- 新增 `frontend-aria2-removal.patch`，删除前端 Aria2 RPC 本地设置和设置组。
- 新增 `scripts/prune-frontend-i18n.mjs`，删除已移除驱动和功能的简体中文词典；
  驱动词典只保留 Local、Dropbox 和 S3。
- 前端构建新增移除功能产物检查，防止升级上游时重新引入离线下载工具。
- 从实际工作区删除 88 个已空置的驱动、分享、离线下载、Torrent 和跨平台构建
  目录。
- 新增本文档并从 `README.md` 提供维护入口。

本次实际验证：

- 三个前端补丁、Aria2 清理补丁和词典裁剪脚本均成功应用到固定前端提交。
- Vite 生产构建成功；裁剪词典后主应用 `store-*.js` 从约 575 KB 降至约
  525 KB。
- 最终前端产物中没有 Aria2 字符串，应用词典 bundle 中没有离线下载、
  qBittorrent 或 Transmission 字符串。
- `server/middlewares`、驱动注册、Local、Dropbox 和 S3 核心测试通过。
- Go 1.26.4 成功生成静态链接的 Linux arm64 ELF，且 `version` 命令可运行。
- Bash 脚本语法、GitHub Actions YAML 和 `git diff --check` 通过。
- `go test ./...` 在当前环境未全通过，失败项为缺少 FUSE/CGO 定义，以及执行环境
  注入 HTTP 安全传输层导致的代理类型断言；未将其记录为全量测试通过。

## 上游升级步骤

1. 创建升级分支并记录当前可用二进制与测试结果。
2. 阅读上游从当前基线到目标版本的认证、路由、驱动和数据库变更。
3. 合并后重新检查被删除的目录、路由、设置键和依赖没有恢复。
4. 更新固定前端提交与词典，逐个重放并修正四个前端补丁。
5. 构建前端并检查产物中没有已删除功能的入口或文案。
6. 运行核心测试和 Linux arm64 构建。
7. 使用临时数据目录验证首次启动、管理员密码、普通用户隔离和三个存储驱动。
8. 人工审查差异后再发布；不要把自动构建结果视为人工安全审查。

## 维护原则

- 每一处新增代码都应服务于当前保留能力。
- 不为未来可能恢复的功能保留占位实现。
- 删除功能时同步清理路由、设置、依赖、文档、前端词典和发布流程。
- 不删除由保留功能共享的通用基础设施。
- 不在未经实际执行时声称测试、验证或发布成功。
