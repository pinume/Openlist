//go:build linux

package local

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func (d *Local) copyNamedPipe(dstPath string, mode os.FileMode) error {
	parent := filepath.Dir(dstPath)
	if err := d.root.MkdirAll(parent, os.FileMode(d.mkdirPerm)); err != nil {
		return err
	}
	dir, err := d.root.Open(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	return unix.Mkfifoat(int(dir.Fd()), filepath.Base(dstPath), uint32(mode.Perm()))
}
