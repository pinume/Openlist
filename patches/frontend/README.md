# 历史 patch（已并入 web/）

这里的 patch 文件曾用于在构建时对上游 `OpenList-Frontend` 打补丁。自
2026-07-30 起，前端源码已在应用全部补丁后固化提交到仓库根目录的 `web/`，
构建脚本 `build-frontend-tinylist.sh` 不再克隆上游或应用这些 patch。

保留这些文件仅作为迁移历史参考。确认新流程（`web/` + `web/UPSTREAM.md` +
`scripts/sync-frontend.sh`）稳定后，可以删除本目录。
