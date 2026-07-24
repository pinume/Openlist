# OpenList Personal Server

面向个人与小团队的私有多用户文件服务器，基于 OpenList 裁剪。

## 功能范围

保留：

- 用户、登录认证、会话与权限管理
- 仅简体中文网页界面
- 仅面向现代浏览器，不生成旧浏览器兼容资源
- Local、Dropbox、S3（兼容 AWS S3、R2、MinIO）
- 文件浏览、搜索、上传、下载、移动、复制、删除
- 图片、音视频、PDF、Markdown、Office 等文件预览
- WebDAV、FTP、SFTP
- SQLite、MySQL、PostgreSQL
- Linux 原生 `arm64` 单二进制部署

移除：

- 游客与匿名访问
- 公开分享及分享链接
- 离线下载及相关下载工具
- 配置备份与还原
- 管理端“关于”和“文档”入口
- 管理端设置、元信息、索引和任务导航入口
- Local、Dropbox、S3 之外的存储驱动

所有文件访问都需要登录，并同时校验用户基础路径与目录权限。

## 构建

需要 Go、Node.js、Corepack、Git，以及可访问 GitHub 和 npm registry 的网络。

先构建裁剪后的前端：

```bash
./build-frontend-personal.sh
```

再构建 Linux 二进制：

```bash
./build-personal.sh
```

在当前 AArch64 主机上，输出位于 `dist/openlist-linux-arm64`。项目仅支持在
Linux 上构建和原生部署。

## 运行

```bash
./dist/openlist-linux-arm64 server --data ./data
```

首次启动后，使用命令行设置管理员密码：

```bash
./dist/openlist-linux-arm64 admin set NEW_PASSWORD --data ./data
```

默认监听 `5244` 端口。生产环境建议使用 Caddy 或 Nginx 提供 HTTPS，并禁止绕过反向代理直接访问服务端口。

## systemd

仓库提供了 [`deploy/openlist.service`](deploy/openlist.service)。将二进制安装为 `/opt/openlist/openlist`，按实际运行用户调整服务文件后执行：

```bash
sudo cp deploy/openlist.service /etc/systemd/system/openlist.service
sudo systemctl daemon-reload
sudo systemctl enable --now openlist
```

## 前端维护

前端构建固定到上游 OpenList-Frontend 的特定提交，并依次应用
[`frontend-personal.patch`](frontend-personal.patch)、
[`frontend-chinese-ui.patch`](frontend-chinese-ui.patch)、
[`frontend-size.patch`](frontend-size.patch) 和
[`frontend-aria2-removal.patch`](frontend-aria2-removal.patch)。构建脚本校验并提取
同版本官方简体中文字典，其他语言、语言切换入口、离线下载工具残留和旧浏览器兼容
资源不会打包。更新上游前端时，应先确认补丁仍能应用，再完成 TypeScript 与生产
构建检查。

更完整的部署说明见 [`PERSONAL_SERVER.md`](PERSONAL_SERVER.md)。
精简边界、升级流程和验证清单见 [`MAINTENANCE.md`](MAINTENANCE.md)。

## License

本项目沿用上游的 [AGPL-3.0](LICENSE) 许可证。
