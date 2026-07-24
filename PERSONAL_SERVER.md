# OpenList Personal Server

这是一个面向私有多用户场景的 OpenList 精简版本。

## 功能边界

- 文件列表、文件详情、下载、预览、压缩包访问、WebDAV、FTP 和 SFTP 均要求认证。
- 关闭公开分享、永久公开链接、匿名 FTP/SFTP 和游客 API 访问。
- 仅包含 Local、Dropbox 和 S3 三种存储驱动。
- Dropbox 与 S3 下载强制经过 OpenList 代理，使私有下载路由能够检查登录状态和用户根目录。
- 不包含离线下载和 Torrent 专用集成。
- 管理端不包含分享、备份与还原、关于和文档入口。
- 文件内的 PDF、Markdown、Office 等文档预览仍然保留。

静态资源、登录接口、SSO 回调、登录页需要的公开站点设置以及健康检查仍允许匿名访问，以保证客户端能够展示登录页并完成认证。

## 用户隔离

沿用 OpenList 现有用户模型：

- `role` 区分管理员和普通用户。
- `base_path` 将用户限制在指定目录树中。
- `permission` 控制上传、重命名、移动、复制、删除、WebDAV、FTP/SFTP 和压缩包操作。
- Meta 的读写用户列表可以进一步限制具体目录。

管理员集中配置 Local、Dropbox 和 S3 挂载，再通过管理 API 或管理界面设置用户的 `base_path` 和权限。

## 构建

Go 版本和工具链以 `go.mod` 为准。前端需要 Node.js、Corepack 和 Git。

```bash
./build-frontend-personal.sh
```

该脚本从固定的 OpenList-Frontend 上游提交构建，应用仓库中的
`frontend-personal.patch`，并把产物放入 `public/dist`。

随后构建原生 Linux 二进制：

```bash
./build-personal.sh
```

构建脚本只能在 Linux 上运行，并生成当前宿主架构的 Linux 二进制。在
AArch64 主机上，输出为 `dist/openlist-linux-arm64`。

## Linux 安装

创建服务账号和目录：

```bash
sudo useradd --system --home /opt/openlist --shell /usr/sbin/nologin openlist
sudo install -d -o openlist -g openlist -m 0750 /opt/openlist/data
sudo install -o root -g root -m 0755 openlist /opt/openlist/openlist
sudo install -o root -g root -m 0644 deploy/openlist.service /etc/systemd/system/openlist.service
```

首次启动会生成管理员密码，并且只输出到服务控制台。也可以提前指定：

```bash
sudo systemctl edit openlist
```

```ini
[Service]
Environment=OPENLIST_ADMIN_PASSWORD=请替换为高强度密码
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now openlist
sudo journalctl -u openlist -f
```

首次启动会创建 `/opt/openlist/data/config.json`。默认 HTTP 监听地址为 `0.0.0.0:5244`。建议使用 Caddy 或 Nginx 提供 TLS，并通过主机防火墙限制外部直接访问 5244 端口。
