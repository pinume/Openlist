package sign

import (
	"errors"
	"testing"
	"time"
)

func TestHMACSignVerify(t *testing.T) {
	signer := HMACSign{SecretKey: []byte("test-secret")}
	expires := time.Now().Add(time.Hour).Unix()
	valid := signer.Sign("/private/file.txt", expires)
	sameLengthInvalid := "A" + valid[1:]
	if valid[0] == 'A' {
		sameLengthInvalid = "B" + valid[1:]
	}

	tests := []struct {
		name string
		data string
		sign string
		want error
	}{
		{
			name: "valid signature",
			data: "/private/file.txt",
			sign: valid,
		},
		{
			name: "different data",
			data: "/private/other.txt",
			sign: valid,
			want: ErrSignInvalid,
		},
		{
			name: "different signature with same length",
			data: "/private/file.txt",
			sign: sameLengthInvalid,
			want: ErrSignInvalid,
		},
		{
			name: "different signature length",
			data: "/private/file.txt",
			sign: valid[1:],
			want: ErrSignInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := signer.Verify(tt.data, tt.sign)
			if !errors.Is(err, tt.want) {
				t.Errorf("Verify() error = %v, want %v", err, tt.want)
			}
		})
	}
}
