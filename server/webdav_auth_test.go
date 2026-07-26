package server

import (
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWebDAVLoginMigratesLegacyPasswordHash(t *testing.T) {
	conf.Conf = conf.DefaultConfig(t.TempDir())
	testDB, err := gorm.Open(sqlite.Open("file:webdav_auth?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("open WebDAV auth test database: %v", err)
	}
	db.Init(testDB)

	const password = "legacy webdav password"
	staticHash := model.StaticHash(password)
	user := &model.User{
		Username: "legacy-webdav",
		BasePath: "/",
		Salt:     "legacy-webdav-salt",
		PwdHash:  model.HashPwd(staticHash, "legacy-webdav-salt"),
		PwdTS:    1234,
	}
	if err := op.CreateUser(user); err != nil {
		t.Fatalf("create legacy WebDAV user: %v", err)
	}

	if _, ok := tryLogin(user.Username, password); !ok {
		t.Fatal("tryLogin() rejected the legacy password")
	}
	stored, err := db.GetUserByName(user.Username)
	if err != nil {
		t.Fatalf("load migrated WebDAV user: %v", err)
	}
	if stored.NeedsPasswordRehash() {
		t.Fatal("WebDAV login did not migrate the legacy password hash")
	}
	if stored.PwdTS != 1234 {
		t.Fatalf("WebDAV migration changed password timestamp to %d", stored.PwdTS)
	}
	if err := stored.ValidateRawPassword(password); err != nil {
		t.Fatalf("migrated WebDAV password no longer validates: %v", err)
	}
}
