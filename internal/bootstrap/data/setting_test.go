package data

import (
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

func TestDefaultAnnouncementIsEmpty(t *testing.T) {
	for _, item := range initialSettingsForTest(t) {
		if item.Key != conf.Announcement {
			continue
		}
		if item.Value != "" {
			t.Fatalf("default announcement = %q, want empty", item.Value)
		}
		if item.MigrationValue != defaultAnnouncement {
			t.Fatalf("announcement migration value = %q, want previous default", item.MigrationValue)
		}
		return
	}
	t.Fatal("announcement setting not found")
}

func TestDefaultSiteTitleIsTinyList(t *testing.T) {
	for _, item := range initialSettingsForTest(t) {
		if item.Key != conf.SiteTitle {
			continue
		}
		if item.Value != "TinyList" {
			t.Fatalf("default site title = %q, want TinyList", item.Value)
		}
		if item.MigrationValue != "OpenList" {
			t.Fatalf("site title migration value = %q, want OpenList", item.MigrationValue)
		}
		return
	}
	t.Fatal("site title setting not found")
}

func TestDefaultBrandAssetsDoNotUseOpenListLogo(t *testing.T) {
	want := map[string]string{
		conf.Logo:    "favicon.ico",
		conf.Favicon: "",
	}
	for _, item := range initialSettingsForTest(t) {
		value, ok := want[item.Key]
		if !ok {
			continue
		}
		if item.Value != value {
			t.Fatalf("%s default = %q, want %q", item.Key, item.Value, value)
		}
		if item.MigrationValue != defaultOpenListLogo {
			t.Fatalf("%s migration value = %q, want previous OpenList logo", item.Key, item.MigrationValue)
		}
		delete(want, item.Key)
	}
	if len(want) != 0 {
		t.Fatalf("brand settings not found: %v", want)
	}
}

func initialSettingsForTest(t *testing.T) []model.SettingItem {
	t.Helper()
	oldConf := conf.Conf
	conf.Conf = conf.DefaultConfig(t.TempDir())
	t.Cleanup(func() {
		conf.Conf = oldConf
	})
	return InitialSettings()
}
