package local

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootPath
	DirectorySize  bool   `json:"directory_size" default:"false" help:"This might impact host performance"`
	ShowHidden     bool   `json:"show_hidden" default:"true" required:"false" help:"show hidden directories and files"`
	MkdirPerm      string `json:"mkdir_perm" default:"777" help:"The permission bits (octal) for new directories, e.g. 777"`
	RecycleBinPath string `json:"recycle_bin_path" default:"delete permanently" help:"path to recycle bin, delete permanently if empty or keep 'delete permanently'"`
}

var config = driver.Config{
	Name:        "Local",
	LocalSort:   true,
	OnlyProxy:   true,
	NoCache:     true,
	DefaultRoot: "/",
	NoLinkURL:   true,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Local{
			directoryMap: DirectoryMap{},
		}
	})
}
