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
- SQLite
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

## License

本项目沿用上游的 [AGPL-3.0](LICENSE) 许可证。
