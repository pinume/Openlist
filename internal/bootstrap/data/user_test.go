package data

import (
	"strings"
	"testing"
)

func TestNewInitialAdminUsesArgon2id(t *testing.T) {
	const password = "initial secret"
	admin := newInitialAdmin(password)
	if !strings.HasPrefix(admin.PwdHash, "$argon2id$") {
		t.Fatalf("initial admin password hash = %q, want Argon2id", admin.PwdHash)
	}
	if err := admin.ValidateRawPassword(password); err != nil {
		t.Fatalf("initial admin password does not validate: %v", err)
	}
}
