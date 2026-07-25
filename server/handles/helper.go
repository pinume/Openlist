package handles

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/public"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// CurrentUser returns the authenticated user stored by the auth middleware.
// A missing or invalid user is treated as an authentication failure instead of
// allowing a handler to panic on a type assertion.
func CurrentUser(c *gin.Context) (*model.User, bool) {
	user, ok := common.UserFromContext(c.Request.Context())
	if !ok {
		common.ErrorStrResp(c, "Authentication required", 401)
		c.Abort()
		return nil, false
	}
	return user, true
}

// resolveAndAuthorize resolves a user-relative request path and applies the
// shared meta/password permission check. On failure it writes the response.
func resolveAndAuthorize(c *gin.Context, user *model.User, reqPath, password string) (string, *model.Meta, bool) {
	resolvedPath, err := user.JoinPath(reqPath)
	if err != nil {
		common.ErrorResp(c, err, 403)
		return "", nil, false
	}
	meta, ok := authorizeResolvedPath(c, user, resolvedPath, password)
	return resolvedPath, meta, ok
}

// authorizeResolvedPath applies the shared authorization flow to a path that
// has already been resolved, such as an administrator's force-root request.
func authorizeResolvedPath(c *gin.Context, user *model.User, resolvedPath, password string) (*model.Meta, bool) {
	meta, err := op.GetNearestMeta(resolvedPath)
	if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
		common.ErrorResp(c, err, 500, true)
		return nil, false
	}
	common.GinAppendValues(c, conf.MetaKey, meta)
	if !common.CanAccess(user, meta, resolvedPath, password) {
		common.ErrorStrResp(c, "password is incorrect or you have no permission", 403)
		return nil, false
	}
	return meta, true
}

func Favicon(c *gin.Context) {
	favicon := setting.GetStr(conf.Favicon)
	if favicon != "" {
		c.Redirect(302, favicon)
		return
	}
	logo, err := public.Public.ReadFile("tinylist.svg")
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(200, "image/svg+xml", logo)
}

func Robots(c *gin.Context) {
	c.String(200, setting.GetStr(conf.RobotsTxt))
}

func Plist(c *gin.Context) {
	linkNameB64 := strings.TrimSuffix(c.Param("link_name"), ".plist")
	linkName, err := utils.SafeAtob(linkNameB64)
	if err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	linkNameSplit := strings.Split(linkName, "/")
	if len(linkNameSplit) != 2 {
		common.ErrorStrResp(c, "malformed link", 400)
		return
	}
	linkEncode := linkNameSplit[0]
	linkStr, err := url.PathUnescape(linkEncode)
	if err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	link, err := url.Parse(linkStr)
	if err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	nameEncode := linkNameSplit[1]
	fullName, err := url.PathUnescape(nameEncode)
	if err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	name := fullName
	identifier := fmt.Sprintf("org.oplist.%s", fullName)
	if strings.Contains(fullName, "@") {
		ss := strings.Split(fullName, "@")
		name = strings.Join(ss[:len(ss)-1], "@")
		identifier = ss[len(ss)-1]
	}
	Url := link.String()
	Url = strings.ReplaceAll(Url, "<", "&lt;")
	Url = strings.ReplaceAll(Url, ">", "&gt;")
	name = html.EscapeString(name)
	identifier = html.EscapeString(identifier)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>items</key>
        <array>
            <dict>
                <key>assets</key>
                <array>
                    <dict>
                        <key>kind</key>
                        <string>software-package</string>
                        <key>url</key>
                        <string><![CDATA[%s]]></string>
                    </dict>
                </array>
                <key>metadata</key>
                <dict>
                    <key>bundle-identifier</key>
					<string>%s</string>
					<key>bundle-version</key>
                    <string>4.4</string>
                    <key>kind</key>
                    <string>software</string>
                    <key>title</key>
                    <string>%s</string>
                </dict>
            </dict>
        </array>
    </dict>
</plist>`, Url, identifier, name)
	c.Header("Content-Type", "application/xml;charset=utf-8")
	c.Status(200)
	_, _ = c.Writer.WriteString(plist)
}
