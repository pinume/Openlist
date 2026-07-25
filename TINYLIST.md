# TinyList

这是一个面向私有多用户场景的 TinyList 私有文件服务器，基于
[OpenList](https://github.com/OpenListTeam/OpenList) 精简。

## 功能边界

- 文件列表、文件详情、下载和 WebDAV 均要求认证。
- 不包含游客角色、内置 `guest` 账号或匿名 API 访问。
- 仅包含 Local 和 Dropbox 两种存储驱动。
- Dropbox 下载强制经过 TinyList 代理，使私有下载路由能够检查登录状态和用户根目录。
- 不包含离线下载和 Torrent 专用集成。
- 不包含需要 CGO/libfuse 的 FUSE 本地挂载能力。
- 管理端不包含分享、备份与还原、关于和文档入口。
- 不提供图片、音视频、PDF、Markdown、Office、压缩包等文件预览；打开文件会直接下载。
- 不生成或展示文件缩略图。
- 支持在浏览器中流式打包下载整个文件夹，不在服务端生成完整 ZIP 临时文件。

静态资源、登录接口、SSO 回调、登录页需要的公开站点设置以及健康检查仍允许
匿名访问，以保证客户端能够展示登录页并完成认证。用户模型只保留管理员和普通
用户；启动时会清理旧数据库中角色编号为 `1` 的历史游客记录。

## 用户隔离

沿用上游 OpenList 的用户模型：

- `role` 区分管理员和普通用户。
- `base_path` 将用户限制在指定目录树中。
- `permission` 控制上传、重命名、移动、复制、删除、WebDAV 和压缩包操作。
- Meta 的读写用户列表可以进一步限制具体目录。

管理员集中配置 Local、Dropbox 挂载，再通过管理 API 或管理界面设置用户的 `base_path` 和权限。

## 构建

Go 版本和工具链以 `go.mod` 为准。前端需要 Node.js、Corepack 和 Git。

```bash
./build-frontend-tinylist.sh
```

该脚本从固定的 OpenList-Frontend 上游提交构建，应用仓库中的
前端裁剪补丁，并把产物放入 `public/dist`。

随后构建原生 Linux 二进制：

```bash
./build-tinylist.sh
```

构建脚本只能在 Linux 上运行，并生成当前宿主架构的 Linux 二进制。在
AArch64 主机上，输出为 `dist/tinylist-linux-arm64`。

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
