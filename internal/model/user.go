package model

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils/random"
	"github.com/OpenListTeam/go-cache"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pkg/errors"
	"golang.org/x/crypto/argon2"
)

const (
	GENERAL = 0
	ADMIN   = 2
)

const (
	StaticHashSalt = "https://github.com/alist-org/alist"

	InvalidUsernameOrPassword = "Invalid username or password"
	Invalid2FACode            = "Invalid 2FA code"
	TooManyAttempts           = "Too many unsuccessful sign-in attempts have been made using an incorrect username or password, Try again later."
)

const (
	argon2Memory      = 19 * 1024
	argon2Iterations  = 2
	argon2Parallelism = 1
	argon2KeyLength   = 32
	argon2Prefix      = "$argon2id$v=19$m=19456,t=2,p=1$"
)

var LoginCache = cache.NewMemCache[int]()

var (
	DefaultLockDuration   = time.Minute * 5
	DefaultMaxAuthRetries = 5
)

type User struct {
	ID       uint   `json:"id" gorm:"primaryKey"`                      // unique key
	Username string `json:"username" gorm:"unique" binding:"required"` // username
	PwdHash  string `json:"-"`                                         // password hash
	PwdTS    int64  `json:"-"`                                         // password timestamp
	Salt     string `json:"-"`                                         // unique salt
	Password string `json:"password"`                                  // password
	BasePath string `json:"base_path"`                                 // base path
	Role     int    `json:"role"`                                      // user's role
	Disabled bool   `json:"disabled"`
	// Determine permissions by bit
	//   0:  can see hidden files
	//   1:  can access without password
	//   2:  reserved
	//   3:  can mkdir and upload
	//   4:  can rename
	//   5:  can move
	//   6:  can copy
	//   7:  can remove
	//   8:  webdav read
	//   9:  webdav write
	//   10-11: reserved
	//   12: can read archives
	//   13: can decompress archives
	Permission int32  `json:"permission"`
	OtpSecret  string `json:"-"`
	SsoID      string `json:"sso_id"` // unique by sso platform
	Authn      string `gorm:"type:text" json:"-"`
	AllowLdap  bool   `json:"allow_ldap" gorm:"default:true"`
}

func (u *User) IsAdmin() bool {
	return u.Role == ADMIN
}

func (u *User) ValidateRawPassword(password string) error {
	return u.ValidatePwdStaticHash(StaticHash(password))
}

func (u *User) ValidatePwdStaticHash(pwdStaticHash string) error {
	if pwdStaticHash == "" {
		return errors.WithStack(errs.EmptyPassword)
	}
	if strings.HasPrefix(u.PwdHash, "$argon2id$") {
		if !validateArgon2Hash(pwdStaticHash, u.PwdHash) {
			return errors.WithStack(errs.WrongPassword)
		}
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(u.PwdHash), []byte(HashPwd(pwdStaticHash, u.Salt))) != 1 {
		return errors.WithStack(errs.WrongPassword)
	}
	return nil
}

func (u *User) SetPassword(pwd string) *User {
	return u.SetPasswordStaticHash(StaticHash(pwd))
}

func (u *User) SetPasswordStaticHash(pwdStaticHash string) *User {
	u.Salt = random.String(16)
	u.PwdHash = encodeArgon2Hash(pwdStaticHash, u.Salt)
	u.PwdTS = time.Now().Unix()
	return u
}

func (u *User) RehashPasswordStaticHash(pwdStaticHash string) *User {
	passwordTimestamp := u.PwdTS
	u.SetPasswordStaticHash(pwdStaticHash)
	u.PwdTS = passwordTimestamp
	return u
}

func (u *User) NeedsPasswordRehash() bool {
	return !strings.HasPrefix(u.PwdHash, argon2Prefix)
}

func CanSeeHides(permission int32) bool {
	return permission&1 == 1
}

func (u *User) CanSeeHides() bool {
	return CanSeeHides(u.Permission)
}

func CanAccessWithoutPassword(permission int32) bool {
	return (permission>>1)&1 == 1
}

func (u *User) CanAccessWithoutPassword() bool {
	return CanAccessWithoutPassword(u.Permission)
}

func CanWriteContent(permission int32) bool {
	return (permission>>3)&1 == 1
}

func (u *User) CanWriteContent() bool {
	return CanWriteContent(u.Permission)
}

func CanRename(permission int32) bool {
	return (permission>>4)&1 == 1
}

func (u *User) CanRename() bool {
	return CanRename(u.Permission)
}

func CanMove(permission int32) bool {
	return (permission>>5)&1 == 1
}

func (u *User) CanMove() bool {
	return CanMove(u.Permission)
}

func CanCopy(permission int32) bool {
	return (permission>>6)&1 == 1
}

func (u *User) CanCopy() bool {
	return CanCopy(u.Permission)
}

func CanRemove(permission int32) bool {
	return (permission>>7)&1 == 1
}

func (u *User) CanRemove() bool {
	return CanRemove(u.Permission)
}

func CanWebdavRead(permission int32) bool {
	return (permission>>8)&1 == 1
}

func (u *User) CanWebdavRead() bool {
	return CanWebdavRead(u.Permission)
}

func CanWebdavManage(permission int32) bool {
	return (permission>>9)&1 == 1
}

func (u *User) CanWebdavManage() bool {
	return CanWebdavManage(u.Permission)
}

func CanReadArchives(permission int32) bool {
	return (permission>>12)&1 == 1
}

func (u *User) CanReadArchives() bool {
	return CanReadArchives(u.Permission)
}

func CanDecompress(permission int32) bool {
	return (permission>>13)&1 == 1
}

func (u *User) CanDecompress() bool {
	return CanDecompress(u.Permission)
}

func (u *User) JoinPath(reqPath string) (string, error) {
	return utils.JoinBasePath(u.BasePath, reqPath)
}

func StaticHash(password string) string {
	return utils.HashData(utils.SHA256, []byte(fmt.Sprintf("%s-%s", password, StaticHashSalt)))
}

func HashPwd(static string, salt string) string {
	return utils.HashData(utils.SHA256, []byte(fmt.Sprintf("%s-%s", static, salt)))
}

func TwoHashPwd(password string, salt string) string {
	return HashPwd(StaticHash(password), salt)
}

func encodeArgon2Hash(pwdStaticHash, salt string) string {
	hash := argon2.IDKey(
		[]byte(pwdStaticHash),
		[]byte(salt),
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)
	return argon2Prefix +
		base64.RawStdEncoding.EncodeToString([]byte(salt)) + "$" +
		base64.RawStdEncoding.EncodeToString(hash)
}

func validateArgon2Hash(pwdStaticHash, encoded string) bool {
	payload, ok := strings.CutPrefix(encoded, argon2Prefix)
	if !ok {
		return false
	}
	saltEncoded, hashEncoded, ok := strings.Cut(payload, "$")
	if !ok {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(saltEncoded)
	if err != nil || len(salt) < 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(hashEncoded)
	if err != nil || len(expected) != argon2KeyLength {
		return false
	}
	actual := argon2.IDKey(
		[]byte(pwdStaticHash),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (u *User) WebAuthnID() []byte {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, uint64(u.ID))
	return bs
}

func (u *User) WebAuthnName() string {
	return u.Username
}

func (u *User) WebAuthnDisplayName() string {
	return u.Username
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	var res []webauthn.Credential
	err := json.Unmarshal([]byte(u.Authn), &res)
	if err != nil {
		fmt.Println(err)
	}
	return res
}

func (u *User) WebAuthnIcon() string {
	return ""
}
