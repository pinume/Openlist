# 前端源码来源

`web/` 目录下的前端源码固化自上游项目，并已应用 TinyList 的全部裁剪改动。

- 上游仓库：https://github.com/OpenListTeam/OpenList-Frontend
- 基线提交：`0d149d1ac40087556a36efecf11a51c012882e57`（`v4.2.4` 发布提交）
- 上游许可证：MIT（见 `web/LICENSE`，与仓库根目录 `LICENSE` 的 AGPL-3.0
  不同，需要分开保留）
- i18n 语言包：`https://github.com/OpenListTeam/OpenList-Frontend/releases/download/v4.2.4/i18n.tar.gz`
  （sha256 `f969170b947a185baef431dc6dabcfd90ed3826b535438661fdf84d6d076a38b`），
  解出 `zh-CN` 后经 `scripts/prune-frontend-i18n.mjs` 裁剪
- 导入日期：2026-07-30

## 已合并的裁剪内容

以下改动在导入时已直接应用到本目录的源码中（不再以 patch 形式单独维护，
历史 patch 保留在仓库根目录 `patches/frontend/`，仅作参考）：

- `personal.patch`、`chinese-ui.patch`、`size.patch`、`tinylist.patch`、
  `header-logo.patch` —— TinyList 品牌、中文界面、体积/展示定制
- `aria2-removal.patch`、`download-options-removal.patch`、
  `manual-download.patch` —— 移除离线下载，精简下载选项
- `preview-removal.patch`、`preview-dependencies.patch`、
  `preview-build.patch` —— 移除文件预览功能及其依赖
- `folder-download.patch` —— 文件夹流式 ZIP 打包下载
- `service-removal.patch` —— 移除 Service Worker / PWA 相关功能

以及构建脚本原本额外做的文件删除：`src/pages/home/previews/`、
`src/components/artplayer-plugin-ass/`、多余播放器图标、
`EncodingSelect.tsx`/`Markdown.tsx`/`MonacoEditor.tsx`/`markdown.css`、
`Readme.tsx`、`open-with.tsx`、`ImageItem.tsx`/`Images.tsx`、
`S3.tsx`/`S3BucketItem.tsx`/`S3Buckets.tsx`、
`PublicKey.tsx`/`PublicKeys.tsx`/`sshkey.ts`，以及英文 (`en`) 语言包。

## 后续升级上游

不再在构建期自动拉取上游代码。升级前端到新的上游提交时：

1. 用 `scripts/sync-frontend.sh <新 commit>` 生成新提交与当前 `web/` 的差异，
   人工审查改动。
2. 手动把需要的改动应用到 `web/`，保持本文件列出的裁剪内容不被上游改动覆盖。
3. 更新本文件的“基线提交”“i18n 语言包”等字段。
4. 提交一个独立的同步 commit/PR（`build(web): sync frontend to <commit>`），
   不要和功能改动混在一起，方便审查差异。
