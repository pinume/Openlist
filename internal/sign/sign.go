package sign

import (
	"sync/atomic"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/pkg/sign"
)

var instance atomic.Value

const temporaryDownloadLifetime = 5 * time.Minute

func current() sign.Sign {
	if signer := instance.Load(); signer != nil {
		return signer.(sign.Sign)
	}
	Reload()
	return instance.Load().(sign.Sign)
}

func Sign(data string) string {
	return withDuration(data, temporaryDownloadLifetime)
}

func withDuration(data string, d time.Duration) string {
	return current().Sign(data, time.Now().Add(d).Unix())
}

func Verify(data string, sign string) error {
	return current().Verify(data, sign)
}

// Reload atomically replaces the signer after the token setting changes.
func Reload() {
	instance.Store(sign.NewHMACSign([]byte(setting.GetStr(conf.Token))))
}
