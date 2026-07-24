package middlewares

import (
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"

	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const signedDownloadKey = "signed_download"

func PathParse(c *gin.Context) {
	rawPath := parsePath(c.Param("path"))
	common.GinAppendValues(c, conf.PathKey, rawPath)
	c.Next()
}

// DownloadAuth accepts either a normal authenticated request or a signed
// download URL. Browser navigations cannot attach the Authorization header
// stored by the frontend, so the path-bound signature acts as the credential
// for that download only.
func DownloadAuth(verifyFunc func(string, string) error) gin.HandlerFunc {
	auth := Auth(false)
	return func(c *gin.Context) {
		rawPath, ok := c.Request.Context().Value(conf.PathKey).(string)
		if !ok {
			common.ErrorPage(c, errs.PermissionDenied, 401)
			return
		}
		s := strings.TrimSuffix(c.Query("sign"), "/")
		var signErr error
		if s != "" {
			signErr = verifyFunc(rawPath, s)
			if signErr == nil {
				c.Set(signedDownloadKey, true)
				c.Next()
				return
			}
		}
		if c.GetHeader("Authorization") != "" {
			auth(c)
			return
		}
		if signErr != nil {
			common.ErrorPage(c, signErr, 401)
			return
		}
		common.ErrorPage(c, errs.PermissionDenied, 401)
	}
}

func Down(verifyFunc func(string, string) error) func(c *gin.Context) {
	return func(c *gin.Context) {
		rawPath := c.Request.Context().Value(conf.PathKey).(string)
		meta, err := op.GetNearestMeta(rawPath)
		if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
			common.ErrorPage(c, err, 500, true)
			return
		}
		common.GinAppendValues(c, conf.MetaKey, meta)
		// verify sign
		if needSign(meta, rawPath) {
			s := c.Query("sign")
			err = verifyFunc(rawPath, strings.TrimSuffix(s, "/"))
			if err != nil {
				common.ErrorPage(c, err, 401)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// UserPathAccess prevents authenticated users from bypassing their base path or
// per-directory read restrictions by crafting a direct download URL.
func UserPathAccess(c *gin.Context) {
	if signed, ok := c.Get(signedDownloadKey); ok && signed == true {
		c.Next()
		return
	}
	user, ok := c.Request.Context().Value(conf.UserKey).(*model.User)
	if !ok || user == nil {
		common.ErrorPage(c, errs.PermissionDenied, 401)
		return
	}
	rawPath := c.Request.Context().Value(conf.PathKey).(string)
	if !utils.IsSubPath(user.BasePath, rawPath) {
		common.ErrorPage(c, errs.PermissionDenied, 403)
		return
	}
	meta, _ := c.Request.Context().Value(conf.MetaKey).(*model.Meta)
	if meta != nil && !common.CanAccess(user, meta, rawPath, meta.Password) {
		common.ErrorPage(c, errs.PermissionDenied, 403)
		return
	}
	c.Next()
}

// TODO: implement
// path maybe contains # ? etc.
func parsePath(path string) string {
	return utils.FixAndCleanPath(path)
}

func needSign(meta *model.Meta, path string) bool {
	if setting.GetBool(conf.SignAll) {
		return true
	}
	if common.IsStorageSignEnabled(path) {
		return true
	}
	if meta == nil || meta.Password == "" {
		return false
	}
	if !meta.PSub && path != meta.Path {
		return false
	}
	return true
}
