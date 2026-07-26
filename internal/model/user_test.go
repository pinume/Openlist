package model

import (
	"strings"
	"testing"
)

func TestSetPasswordUsesArgon2id(t *testing.T) {
	user := &User{}
	user.SetPassword("correct horse battery staple")

	if !strings.HasPrefix(user.PwdHash, argon2Prefix) {
		t.Fatalf("SetPassword() hash = %q, want Argon2id PHC format", user.PwdHash)
	}
	if user.NeedsPasswordRehash() {
		t.Fatal("new Argon2id password unexpectedly needs rehash")
	}
	if err := user.ValidateRawPassword("correct horse battery staple"); err != nil {
		t.Fatalf("ValidateRawPassword() rejected the correct password: %v", err)
	}
	if err := user.ValidateRawPassword("wrong password"); err == nil {
		t.Fatal("ValidateRawPassword() accepted the wrong password")
	}
}

func TestLegacyPasswordHashRemainsCompatible(t *testing.T) {
	const password = "legacy password"
	staticHash := StaticHash(password)
	user := &User{
		Salt:    "legacy-salt",
		PwdHash: HashPwd(staticHash, "legacy-salt"),
		PwdTS:   1234,
	}

	if err := user.ValidatePwdStaticHash(staticHash); err != nil {
		t.Fatalf("ValidatePwdStaticHash() rejected a legacy hash: %v", err)
	}
	if !user.NeedsPasswordRehash() {
		t.Fatal("legacy password hash was not marked for migration")
	}

	user.RehashPasswordStaticHash(staticHash)
	if user.NeedsPasswordRehash() {
		t.Fatal("migrated password still needs rehash")
	}
	if user.PwdTS != 1234 {
		t.Fatalf("migration changed password timestamp to %d", user.PwdTS)
	}
	if err := user.ValidatePwdStaticHash(staticHash); err != nil {
		t.Fatalf("ValidatePwdStaticHash() rejected a migrated hash: %v", err)
	}
}

func TestMalformedArgon2HashIsRejected(t *testing.T) {
	user := &User{PwdHash: argon2Prefix + "invalid"}
	if err := user.ValidateRawPassword("password"); err == nil {
		t.Fatal("ValidateRawPassword() accepted a malformed Argon2id hash")
	}
}
