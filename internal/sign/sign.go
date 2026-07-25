package sign

import (
	"sync/atomic"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/pkg/sign"
)

var instance atomic.Value

func current() sign.Sign {
	if signer := instance.Load(); signer != nil {
		return signer.(sign.Sign)
	}
	Reload()
	return instance.Load().(sign.Sign)
}

func Sign(data string) string {
	expire := setting.GetInt(conf.LinkExpiration, 0)
	if expire == 0 {
		return NotExpired(data)
	} else {
		return WithDuration(data, time.Duration(expire)*time.Hour)
	}
}

func WithDuration(data string, d time.Duration) string {
	return current().Sign(data, time.Now().Add(d).Unix())
}

func NotExpired(data string) string {
	return current().Sign(data, 0)
}

func Verify(data string, sign string) error {
	return current().Verify(data, sign)
}

// Reload atomically replaces the signer after the token setting changes.
func Reload() {
	instance.Store(sign.NewHMACSign([]byte(setting.GetStr(conf.Token))))
}
