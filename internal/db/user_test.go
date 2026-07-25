package db

import (
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDeleteUsersByRole(t *testing.T) {
	originalDB := db
	t.Cleanup(func() {
		db = originalDB
	})

	testDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db = testDB
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	users := []model.User{
		{Username: "general", Role: model.GENERAL},
		{Username: "legacy-guest", Role: 1},
		{
			Username:   "admin",
			Role:       model.ADMIN,
			Permission: (1 << 10) | (1 << 11) | (1 << 12),
		},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := DeleteUsersByRole(1); err != nil {
		t.Fatalf("delete legacy guest role: %v", err)
	}

	var count int64
	if err := db.Model(&model.User{}).Where("role = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("count legacy guest role: %v", err)
	}
	if count != 0 {
		t.Fatalf("legacy guest role still has %d users", count)
	}
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count remaining users: %v", err)
	}
	if count != 2 {
		t.Fatalf("got %d remaining users, want 2", count)
	}

	if err := ClearUserPermissionBits((1 << 10) | (1 << 11)); err != nil {
		t.Fatalf("clear removed FTP permission bits: %v", err)
	}
	var admin model.User
	if err := db.Where("username = ?", "admin").Take(&admin).Error; err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if admin.Permission != 1<<12 {
		t.Fatalf("got permission %#x, want only archive permission %#x", admin.Permission, 1<<12)
	}
}
