package handles

import (
	"net/url"
	stdpath "path"

	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/fs"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type FsGetDirectUploadInfoReq struct {
	Path     string `json:"path" form:"path"`
	FileName string `json:"file_name" form:"file_name"`
	FileSize int64  `json:"file_size" form:"file_size"`
	Tool     string `json:"tool" form:"tool"`
}

// FsGetDirectUploadInfo returns the direct upload info if supported by the driver
// If the driver does not support direct upload, returns null for upload_info
func FsGetDirectUploadInfo(c *gin.Context) {
	var req FsGetDirectUploadInfoReq
	if err := c.ShouldBind(&req); err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	// Decode path
	path, err := url.PathUnescape(req.Path)
	if err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	// Get user and join path
	user, ok := CurrentUser(c)
	if !ok {
		return
	}
	path, err = user.JoinPath(path)
	if err != nil {
		common.ErrorResp(c, err, 403)
		return
	}
	if err := checkRelativePath(req.FileName); err != nil {
		common.ErrorResp(c, err, 403)
		return
	}
	parentMeta, err := op.GetNearestMeta(path)
	if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
		common.ErrorResp(c, err, 500, true)
		return
	}
	if !user.CanWriteContent() && !common.CanWriteContentBypassUserPerms(parentMeta, path) {
		common.ErrorResp(c, errs.PermissionDenied, 403)
		return
	}
	if !common.CanWrite(user, parentMeta, path) {
		common.ErrorResp(c, errs.PermissionDenied, 403)
		return
	}
	overwrite := c.GetHeader("Overwrite") != "false"
	dstPath := stdpath.Join(path, req.FileName)
	if !canWritePath(c, user, dstPath) {
		return
	}
	if !overwrite {
		res, err := fs.Get(c.Request.Context(), dstPath, &fs.GetArgs{NoLog: true})
		if err != nil && !errs.IsObjectNotFound(err) {
			common.ErrorResp(c, err, 500)
			return
		}
		if res != nil {
			common.ErrorStrResp(c, "file exists", 403)
			return
		}
	}
	directUploadInfo, err := fs.GetDirectUploadInfo(c, req.Tool, path, req.FileName, req.FileSize, overwrite)
	if err != nil {
		if !overwrite && errs.IsObjectAlreadyExists(err) {
			common.ErrorStrResp(c, "file exists", 403)
			return
		}
		common.ErrorResp(c, err, 500)
		return
	}
	common.SuccessResp(c, directUploadInfo)
}
