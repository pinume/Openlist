package drivers

import (
	"slices"
	"sort"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

func TestPersonalBuildRegistersOnlySupportedDrivers(t *testing.T) {
	got := op.GetDriverNames()
	sort.Strings(got)
	want := []string{"Dropbox", "Local"}
	if !slices.Equal(got, want) {
		t.Fatalf("registered drivers = %v, want %v", got, want)
	}
}
