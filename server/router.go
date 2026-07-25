package server

import (
	"github.com/OpenListTeam/OpenList/v4/cmd/flags"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/message"
	"github.com/OpenListTeam/OpenList/v4/internal/sign"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/OpenListTeam/OpenList/v4/server/handles"
	"github.com/OpenListTeam/OpenList/v4/server/middlewares"
	"github.com/OpenListTeam/OpenList/v4/server/static"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Init(e *gin.Engine) {
	e.ContextWithFallback = true
	if !utils.SliceContains([]string{"", "/"}, conf.URL.Path) {
		e.GET("/", func(c *gin.Context) {
			c.Redirect(302, conf.URL.Path)
		})
	}
	Cors(e)
	g := e.Group(conf.URL.Path)
	g.Any("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})
	g.GET("/favicon.ico", handles.Favicon)
	g.GET("/robots.txt", handles.Robots)
	g.GET("/manifest.json", static.ManifestJSON)
	common.SecretKey = []byte(conf.Conf.JwtSecret)
	g.Use(middlewares.StoragesLoaded)
	if conf.Conf.MaxConnections > 0 {
		g.Use(middlewares.MaxAllowed(conf.Conf.MaxConnections))
	}
	WebDav(g.Group("/dav"))

	downloadLimiter := middlewares.DownloadRateLimiter(stream.ClientDownloadLimit)
	signCheck := middlewares.Down(sign.Verify)
	downloadAuth := middlewares.DownloadAuth(sign.Verify)
	g.GET("/d/*path", middlewares.PathParse, downloadAuth, signCheck, middlewares.UserPathAccess, downloadLimiter, handles.Down)
	g.GET("/p/*path", middlewares.PathParse, downloadAuth, signCheck, middlewares.UserPathAccess, downloadLimiter, handles.Proxy)
	g.HEAD("/d/*path", middlewares.PathParse, downloadAuth, signCheck, middlewares.UserPathAccess, handles.Down)
	g.HEAD("/p/*path", middlewares.PathParse, downloadAuth, signCheck, middlewares.UserPathAccess, handles.Proxy)
	api := g.Group("/api")
	auth := api.Group("", middlewares.Auth())
	webauthn := api.Group("/authn", middlewares.Authn)

	api.POST("/auth/login", handles.Login)
	api.POST("/auth/login/hash", handles.LoginHash)
	api.POST("/auth/login/ldap", handles.LoginLdap)
	auth.GET("/me", handles.CurrentUserInfo)
	auth.POST("/me/update", handles.UpdateCurrent)
	auth.POST("/auth/2fa/generate", handles.Generate2FA)
	auth.POST("/auth/2fa/verify", handles.Verify2FA)
	auth.GET("/auth/logout", handles.LogOut)

	// auth
	api.GET("/auth/sso", handles.SSOLoginRedirect)
	api.GET("/auth/sso_callback", handles.SSOLoginCallback)
	api.GET("/auth/get_sso_id", handles.SSOLoginCallback)
	api.GET("/auth/sso_get_token", handles.SSOLoginCallback)

	// webauthn
	api.GET("/authn/webauthn_begin_login", handles.BeginAuthnLogin)
	api.POST("/authn/webauthn_finish_login", handles.FinishAuthnLogin)
	webauthn.GET("/webauthn_begin_registration", handles.BeginAuthnRegistration)
	webauthn.POST("/webauthn_finish_registration", handles.FinishAuthnRegistration)
	webauthn.POST("/delete_authn", handles.DeleteAuthnLogin)
	webauthn.GET("/getcredentials", handles.GetAuthnCredentials)

	// no need auth
	public := api.Group("/public")
	public.Any("/settings", handles.PublicSettings)
	public.Any("/archive_extensions", handles.ArchiveExtensions)

	_fs(auth.Group("/fs"))
	fsRead(api.Group("/fs", middlewares.Auth()))
	_task(auth.Group("/task"))
	admin(auth.Group("/admin", middlewares.AuthAdmin))
	if flags.Debug || flags.Dev {
		debug(g.Group("/debug"))
	}
	static.Static(g, func(handlers ...gin.HandlerFunc) {
		e.NoRoute(handlers...)
	})
}

func admin(g *gin.RouterGroup) {
	meta := g.Group("/meta")
	meta.GET("/list", handles.ListMetas)
	meta.GET("/get", handles.GetMeta)
	meta.POST("/create", handles.CreateMeta)
	meta.POST("/update", handles.UpdateMeta)
	meta.POST("/delete", handles.DeleteMeta)

	user := g.Group("/user")
	user.GET("/list", handles.ListUsers)
	user.GET("/get", handles.GetUser)
	user.POST("/create", handles.CreateUser)
	user.POST("/update", handles.UpdateUser)
	user.POST("/cancel_2fa", handles.Cancel2FAById)
	user.POST("/delete", handles.DeleteUser)
	user.POST("/del_cache", handles.DelUserCache)

	storage := g.Group("/storage")
	storage.GET("/list", handles.ListStorages)
	storage.GET("/get", handles.GetStorage)
	storage.POST("/create", handles.CreateStorage)
	storage.POST("/update", handles.UpdateStorage)
	storage.POST("/delete", handles.DeleteStorage)
	storage.POST("/enable", handles.EnableStorage)
	storage.POST("/disable", handles.DisableStorage)
	storage.POST("/load_all", handles.LoadAllStorages)

	driver := g.Group("/driver")
	driver.GET("/list", handles.ListDriverInfo)
	driver.GET("/names", handles.ListDriverNames)
	driver.GET("/info", handles.GetDriverInfo)

	setting := g.Group("/setting")
	setting.GET("/get", handles.GetSetting)
	setting.GET("/list", handles.ListSettings)
	setting.POST("/save", handles.SaveSettings)
	setting.POST("/delete", handles.DeleteSetting)
	setting.POST("/default", handles.DefaultSettings)
	setting.POST("/reset_token", handles.ResetToken)

	// retain /admin/task API to ensure compatibility with legacy automation scripts
	_task(g.Group("/task"))

	ms := g.Group("/message")
	ms.POST("/get", message.HttpInstance.GetHandle)
	ms.POST("/send", message.HttpInstance.SendHandle)

	index := g.Group("/index")
	index.POST("/build", middlewares.SearchIndex, handles.BuildIndex)
	index.POST("/update", middlewares.SearchIndex, handles.UpdateIndex)
	index.POST("/stop", middlewares.SearchIndex, handles.StopIndex)
	index.POST("/clear", middlewares.SearchIndex, handles.ClearIndex)
	index.GET("/progress", middlewares.SearchIndex, handles.GetProgress)

	scan := g.Group("/scan")
	scan.POST("/start", handles.StartManualScan)
	scan.POST("/stop", handles.StopManualScan)
	scan.GET("/progress", handles.GetManualScanProgress)
}

func fsRead(g *gin.RouterGroup) {
	g.Any("/list", handles.FsListSplit)
	g.Any("/get", handles.FsGetSplit)
}

func _fs(g *gin.RouterGroup) {
	g.Any("/search", middlewares.SearchIndex, handles.Search)
	g.Any("/other", handles.FsOther)
	g.Any("/dirs", handles.FsDirs)
	g.POST("/mkdir", handles.FsMkdir)
	g.POST("/rename", handles.FsRename)
	g.POST("/batch_rename", handles.FsBatchRename)
	g.POST("/regex_rename", handles.FsRegexRename)
	g.POST("/move", handles.FsMove)
	g.POST("/recursive_move", handles.FsRecursiveMove)
	g.POST("/copy", handles.FsCopy)
	g.POST("/remove", handles.FsRemove)
	g.POST("/remove_empty_directory", handles.FsRemoveEmptyDirectory)
	uploadLimiter := middlewares.UploadRateLimiter(stream.ClientUploadLimit)
	g.PUT("/put", middlewares.FsUp, uploadLimiter, handles.FsStream)
	g.PUT("/form", middlewares.FsUp, uploadLimiter, handles.FsForm)
	g.POST("/link", middlewares.AuthAdmin, handles.Link)
	g.POST("/archive/decompress", handles.FsArchiveDecompress)
	// Direct upload (client-side upload to storage)
	g.POST("/get_direct_upload_info", middlewares.FsUp, handles.FsGetDirectUploadInfo)
}

func _task(g *gin.RouterGroup) {
	handles.SetupTaskRoute(g)
}

func Cors(r *gin.Engine) {
	config := cors.DefaultConfig()
	// config.AllowAllOrigins = true
	config.AllowOrigins = conf.Conf.Cors.AllowOrigins
	config.AllowHeaders = conf.Conf.Cors.AllowHeaders
	config.AllowMethods = conf.Conf.Cors.AllowMethods
	r.Use(cors.New(config))
}
