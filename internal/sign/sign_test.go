package sign

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

func TestSignUsesTemporaryExpiration(t *testing.T) {
	op.Cache.SetSetting(conf.Token, &model.SettingItem{Key: conf.Token, Value: "temporary-token"})
	Reload()

	before := time.Now().Add(temporaryDownloadLifetime - time.Second).Unix()
	signature := Sign("/private/file.txt")
	separator := strings.LastIndex(signature, ":")
	if separator < 0 {
		t.Fatalf("Sign() returned malformed signature %q", signature)
	}
	expires, err := strconv.ParseInt(signature[separator+1:], 10, 64)
	if err != nil {
		t.Fatalf("Sign() returned invalid expiration: %v", err)
	}
	after := time.Now().Add(temporaryDownloadLifetime + time.Second).Unix()
	if expires < before || expires > after {
		t.Fatalf("Sign() expiration = %d, want between %d and %d", expires, before, after)
	}
}

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
			signature := Sign("/private/file.txt")
			_ = Verify("/private/file.txt", signature)
		}
	}()
	wg.Wait()

	setToken("final-token")
	Reload()
	signature := Sign("/private/file.txt")
	if err := Verify("/private/file.txt", signature); err != nil {
		t.Fatalf("Verify() after final reload returned %v", err)
	}
}
