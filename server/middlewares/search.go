package middlewares

import (
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
)

// SearchIndex gates /api/fs/search. no_index performs a real-time search
// without a persisted index, so it stays available here.
func SearchIndex(c *gin.Context) {
	mode := setting.GetStr(conf.SearchIndex)
	if mode == "none" {
		common.ErrorResp(c, errs.SearchNotAvailable, 404)
		c.Abort()
	} else {
		c.Next()
	}
}

// IndexManage gates the /api/admin/index/* build/update/stop/clear/progress
// routes. no_index has nothing to build or update, so those stay blocked
// alongside none to avoid walking the whole storage for no reason.
func IndexManage(c *gin.Context) {
	mode := setting.GetStr(conf.SearchIndex)
	if mode == "none" || mode == "no_index" {
		common.ErrorResp(c, errs.SearchNotAvailable, 404)
		c.Abort()
	} else {
		c.Next()
	}
}
