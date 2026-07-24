package local

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	stdpath "path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/times"
	log "github.com/sirupsen/logrus"
)

type Local struct {
	model.Storage
	Addition
	mkdirPerm int32

	// directory size data
	directoryMap DirectoryMap
}

func (d *Local) Config() driver.Config {
	return config
}

func (d *Local) Init(ctx context.Context) error {
	if d.MkdirPerm == "" {
		d.mkdirPerm = 0o777
	} else {
		v, err := strconv.ParseUint(d.MkdirPerm, 8, 32)
		if err != nil {
			return err
		}
		d.mkdirPerm = int32(v)
	}
	if !utils.Exists(d.GetRootPath()) {
		return fmt.Errorf("root folder %s not exists", d.GetRootPath())
	}
	if !filepath.IsAbs(d.GetRootPath()) {
		abs, err := filepath.Abs(d.GetRootPath())
		if err != nil {
			return err
		}
		d.Addition.RootFolderPath = abs
	}
	if d.DirectorySize {
		d.directoryMap.root = d.GetRootPath()
		_, err := d.directoryMap.CalculateDirSize(d.GetRootPath())
		if err != nil {
			return err
		}
	} else {
		d.directoryMap.Clear()
	}
	return nil
}

func (d *Local) Drop(ctx context.Context) error {
	return nil
}

func (d *Local) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Local) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	fullPath := dir.GetPath()
	rawFiles, err := readDir(fullPath)
	if d.DirectorySize && args.Refresh {
		d.directoryMap.RecalculateDirSize()
	}
	if err != nil {
		return nil, err
	}
	var files []model.Obj
	for _, f := range rawFiles {
		if d.ShowHidden || !isHidden(f, fullPath) {
			files = append(files, d.FileInfoToObj(f, fullPath))
		}
	}
	return files, nil
}

func (d *Local) FileInfoToObj(f fs.FileInfo, fullPath string) model.Obj {
	isFolder := f.IsDir() || isSymlinkDir(f, fullPath)
	var size int64
	if isFolder {
		node, ok := d.directoryMap.Get(filepath.Join(fullPath, f.Name()))
		if ok {
			size = node.fileSum + node.directorySum
		}
	} else {
		size = f.Size()
	}
	var ctime time.Time
	t, err := times.Stat(stdpath.Join(fullPath, f.Name()))
	if err == nil {
		if t.HasBirthTime() {
			ctime = t.BirthTime()
		}
	}

	file := model.Object{
		Path:     filepath.Join(fullPath, f.Name()),
		Name:     f.Name(),
		Modified: f.ModTime(),
		Size:     size,
		IsFolder: isFolder,
		Ctime:    ctime,
	}
	return &file
}

func (d *Local) Get(ctx context.Context, path string) (model.Obj, error) {
	path = filepath.Join(d.GetRootPath(), path)
	f, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errs.ObjectNotFound
		}
		return nil, err
	}
	isFolder := f.IsDir() || isSymlinkDir(f, path)
	size := f.Size()
	if isFolder {
		node, ok := d.directoryMap.Get(path)
		if ok {
			size = node.fileSum + node.directorySum
		}
	} else {
		size = f.Size()
	}
	var ctime time.Time
	t, err := times.Stat(path)
	if err == nil {
		if t.HasBirthTime() {
			ctime = t.BirthTime()
		}
	}
	file := model.Object{
		Path:     path,
		Name:     f.Name(),
		Modified: f.ModTime(),
		Ctime:    ctime,
		Size:     size,
		IsFolder: isFolder,
	}
	return &file, nil
}

func (d *Local) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	fullPath := file.GetPath()
	link := &model.Link{}
	open, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	link.ContentLength = file.GetSize()
	var MFile model.File = open
	link.SyncClosers.AddIfCloser(MFile)
	link.RangeReader = stream.GetRangeReaderFromMFile(link.ContentLength, MFile)
	link.RequireReference = link.SyncClosers.Length() > 0
	return link, nil
}

func (d *Local) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	fullPath := filepath.Join(parentDir.GetPath(), dirName)
	err := os.MkdirAll(fullPath, os.FileMode(d.mkdirPerm))
	if err != nil {
		return err
	}
	return nil
}

func (d *Local) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	srcPath := srcObj.GetPath()
	dstPath := filepath.Join(dstDir.GetPath(), srcObj.GetName())
	if utils.IsSubPath(srcPath, dstPath) {
		return fmt.Errorf("the destination folder is a subfolder of the source folder")
	}
	err := os.Rename(srcPath, dstPath)
	if isCrossDeviceError(err) {
		// 跨设备移动，变更为移动任务
		return errs.NotImplement
	}
	if err == nil {
		srcParent := filepath.Dir(srcPath)
		dstParent := filepath.Dir(dstPath)
		if d.directoryMap.Has(srcParent) {
			d.directoryMap.UpdateDirSize(srcParent)
			d.directoryMap.UpdateDirParents(srcParent)
		}
		if d.directoryMap.Has(dstParent) {
			d.directoryMap.UpdateDirSize(dstParent)
			d.directoryMap.UpdateDirParents(dstParent)
		}
	}
	return err
}

func (d *Local) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	srcPath := srcObj.GetPath()
	dstPath := filepath.Join(filepath.Dir(srcPath), newName)
	err := os.Rename(srcPath, dstPath)
	if err != nil {
		return err
	}

	if srcObj.IsDir() {
		if d.directoryMap.Has(srcPath) {
			d.directoryMap.DeleteDirNode(srcPath)
			d.directoryMap.CalculateDirSize(dstPath)
		}
	}

	return nil
}

func (d *Local) Copy(_ context.Context, srcObj, dstDir model.Obj) error {
	srcPath := srcObj.GetPath()
	dstPath := filepath.Join(dstDir.GetPath(), srcObj.GetName())
	if utils.IsSubPath(srcPath, dstPath) {
		return fmt.Errorf("the destination folder is a subfolder of the source folder")
	}
	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}
	// 复制regular文件会返回errs.NotImplement, 转为复制任务
	if err = d.tryCopy(srcPath, dstPath, info); err != nil {
		return err
	}

	if d.directoryMap.Has(filepath.Dir(dstPath)) {
		d.directoryMap.UpdateDirSize(filepath.Dir(dstPath))
		d.directoryMap.UpdateDirParents(filepath.Dir(dstPath))
	}

	return nil
}

func (d *Local) Remove(ctx context.Context, obj model.Obj) error {
	var err error
	if utils.SliceContains([]string{"", "delete permanently"}, d.RecycleBinPath) {
		if obj.IsDir() {
			err = os.RemoveAll(obj.GetPath())
		} else {
			err = os.Remove(obj.GetPath())
		}
	} else {
		objPath := obj.GetPath()
		objName := obj.GetName()
		var relPath string
		relPath, err = filepath.Rel(d.GetRootPath(), filepath.Dir(objPath))
		if err != nil {
			return err
		}
		recycleBinPath := filepath.Join(d.RecycleBinPath, relPath)
		if !utils.Exists(recycleBinPath) {
			err = os.MkdirAll(recycleBinPath, 0o755)
			if err != nil {
				return err
			}
		}

		dstPath := filepath.Join(recycleBinPath, objName)
		if utils.Exists(dstPath) {
			dstPath = filepath.Join(recycleBinPath, objName+"_"+time.Now().Format("20060102150405"))
		}
		err = os.Rename(objPath, dstPath)
	}
	if err != nil {
		return err
	}
	if obj.IsDir() {
		if d.directoryMap.Has(obj.GetPath()) {
			d.directoryMap.DeleteDirNode(obj.GetPath())
			d.directoryMap.UpdateDirSize(filepath.Dir(obj.GetPath()))
			d.directoryMap.UpdateDirParents(filepath.Dir(obj.GetPath()))
		}
	} else {
		if d.directoryMap.Has(filepath.Dir(obj.GetPath())) {
			d.directoryMap.UpdateDirSize(filepath.Dir(obj.GetPath()))
			d.directoryMap.UpdateDirParents(filepath.Dir(obj.GetPath()))
		}
	}

	return nil
}

func (d *Local) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	fullPath := filepath.Join(dstDir.GetPath(), stream.GetName())
	var existingMode fs.FileMode
	if info, statErr := os.Stat(fullPath); statErr == nil {
		existingMode = info.Mode().Perm()
	}
	out, err := createUploadTemp(dstDir.GetPath())
	if err != nil {
		return err
	}
	tempPath := out.Name()
	committed := false
	defer func() {
		_ = out.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	err = utils.CopyWithCtx(ctx, out, stream, stream.GetSize(), up)
	if err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if existingMode != 0 {
		if err = os.Chmod(tempPath, existingMode); err != nil {
			return err
		}
	}
	err = os.Chtimes(tempPath, stream.ModTime(), stream.ModTime())
	if err != nil {
		log.Errorf("[local] failed to change time of %s: %s", tempPath, err)
	}
	if err = os.Rename(tempPath, fullPath); err != nil {
		return err
	}
	committed = true
	if d.directoryMap.Has(dstDir.GetPath()) {
		d.directoryMap.UpdateDirSize(dstDir.GetPath())
		d.directoryMap.UpdateDirParents(dstDir.GetPath())
	}

	return nil
}

func (d *Local) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	du, err := getDiskUsage(d.RootFolderPath)
	if err != nil {
		return nil, err
	}
	return &model.StorageDetails{
		DiskUsage: du,
	}, nil
}

var _ driver.Driver = (*Local)(nil)
