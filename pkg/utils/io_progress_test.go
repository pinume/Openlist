package utils

import (
	"bytes"
	"context"
	"testing"
)

func TestCopyWithCtxReportsOnlyPercentageChanges(t *testing.T) {
	data := bytes.Repeat([]byte{0}, 1024*1024)
	var dst bytes.Buffer
	var updates []float64

	err := CopyWithCtx(context.Background(), &dst, bytes.NewReader(data), int64(len(data)), func(value float64) {
		updates = append(updates, value)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst.Bytes(), data) {
		t.Fatal("copied data differs from source")
	}
	if len(updates) > 100 {
		t.Fatalf("updates = %d, want at most 100", len(updates))
	}
	if updates[len(updates)-1] != 100 {
		t.Fatalf("last update = %v, want 100", updates[len(updates)-1])
	}
}
