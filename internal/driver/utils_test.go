package driver

import "testing"

func TestProgressReportsOnlyPercentageChanges(t *testing.T) {
	var updates []float64
	progress := NewProgress(1000, func(value float64) {
		updates = append(updates, value)
	})

	for range 1000 {
		_, _ = progress.Write([]byte{0})
	}

	if len(updates) != 100 {
		t.Fatalf("updates = %d, want 100", len(updates))
	}
	if updates[0] != 1 || updates[len(updates)-1] != 100 {
		t.Fatalf("updates range = %v..%v, want 1..100", updates[0], updates[len(updates)-1])
	}
}
