package sign

import (
	"sync"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

func TestReloadIsSafeDuringConcurrentSigning(t *testing.T) {
	setToken := func(value string) {
		op.Cache.SetSetting(conf.Token, &model.SettingItem{Key: conf.Token, Value: value})
	}

	setToken("initial-token")
	Reload()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			setToken("rotated-token")
			Reload()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			signature := NotExpired("/private/file.txt")
			_ = Verify("/private/file.txt", signature)
		}
	}()
	wg.Wait()

	setToken("final-token")
	Reload()
	signature := NotExpired("/private/file.txt")
	if err := Verify("/private/file.txt", signature); err != nil {
		t.Fatalf("Verify() after final reload returned %v", err)
	}
}
