package local

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/times"
	log "github.com/sirupsen/logrus"
)

type Local struct {
	model.Storage
	Addition
	mkdirPerm int32
	root      *os.Root

	recycleBinPath string

	// directory size data
	directoryMap DirectoryMap
}

func (d *Local) Config() driver.Config {
	return config
}

func (d *Local) Init(ctx context.Context) error {
	if d.root != nil {
		if err := d.root.Close(); err != nil {
			return err
		}
		d.root = nil
	}
	if d.MkdirPerm == "" {
		d.mkdirPerm = 0o777
	} else {
		v, err := strconv.ParseUint(d.MkdirPerm, 8, 32)
		if err != nil {
			return err
		}
		d.mkdirPerm = int32(v)
	}
	if !filepath.IsAbs(d.GetRootPath()) {
		abs, err := filepath.Abs(d.GetRootPath())
		if err != nil {
			return err
		}
		d.Addition.RootFolderPath = abs
	}
	root, err := os.OpenRoot(d.GetRootPath())
	if err != nil {
		return fmt.Errorf("open root folder %s: %w", d.GetRootPath(), err)
	}
	d.root = root
	if !utils.SliceContains([]string{"", "delete permanently"}, d.RecycleBinPath) {
		recycleBinPath := d.RecycleBinPath
		if filepath.IsAbs(recycleBinPath) {
			recycleBinPath, err = filepath.Rel(d.GetRootPath(), recycleBinPath)
			if err != nil {
				_ = d.root.Close()
				d.root = nil
				return err
			}
		}
		d.recycleBinPath, err = cleanRootPath(recycleBinPath)
		if err != nil {
			_ = d.root.Close()
			d.root = nil
			return fmt.Errorf("recycle bin must be inside the storage root: %w", err)
		}
	} else {
		d.recycleBinPath = d.RecycleBinPath
	}
	d.directoryMap.Configure(".", d.root)
	if d.DirectorySize {
		if err = d.directoryMap.RecalculateDirSize(); err != nil {
			_ = d.root.Close()
			d.root = nil
			return err
		}
	} else {
		d.directoryMap.Clear()
	}
	return nil
}

func (d *Local) Drop(ctx context.Context) error {
	if d.root != nil {
		err := d.root.Close()
		d.root = nil
		d.directoryMap.Configure("", nil)
		return err
	}
	return nil
}

func (d *Local) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Local) GetRoot(ctx context.Context) (model.Obj, error) {
	info, err := d.root.Stat(".")
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if node, ok := d.directoryMap.Get("."); ok {
		size = node.fileSum + node.directorySum
	}
	var ctime time.Time
	file, openErr := d.root.Open(".")
	if openErr == nil {
		if t, statErr := times.StatFile(file); statErr == nil && t.HasBirthTime() {
			ctime = t.BirthTime()
		}
		_ = file.Close()
	}
	return &model.Object{
		Path:     ".",
		Name:     op.RootName,
		Modified: info.ModTime(),
		Ctime:    ctime,
		Size:     size,
		IsFolder: true,
		Mask:     model.Locked,
	}, nil
}

func (d *Local) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	parent, err := cleanRootPath(dir.GetPath())
	if err != nil {
		return nil, err
	}
	rawFiles, err := readDir(d.root, parent)
	if err != nil {
		return nil, err
	}
	if d.DirectorySize && args.Refresh {
		d.refreshDirectory(parent)
	}
	var files []model.Obj
	for _, f := range rawFiles {
		if d.ShowHidden || !isHidden(f, parent) {
			files = append(files, d.fileInfoToObj(f, parent))
		}
	}
	return files, nil
}

func (d *Local) fileInfoToObj(f fs.FileInfo, parent string) model.Obj {
	objPath := filepath.Join(parent, f.Name())
	isFolder := f.IsDir() || isSymlinkDir(d.root, f, parent)
	var size int64
	if isFolder {
		node, ok := d.directoryMap.Get(objPath)
		if ok {
			size = node.fileSum + node.directorySum
		}
	} else {
		size = f.Size()
	}
	var ctime time.Time
	file, err := d.root.Open(objPath)
	if err == nil {
		if t, statErr := times.StatFile(file); statErr == nil && t.HasBirthTime() {
			ctime = t.BirthTime()
		}
		_ = file.Close()
	}

	obj := model.Object{
		Path:     objPath,
		Name:     f.Name(),
		Modified: f.ModTime(),
		Size:     size,
		IsFolder: isFolder,
		Ctime:    ctime,
	}
	return &obj
}

func (d *Local) Get(ctx context.Context, path string) (model.Obj, error) {
	path, err := cleanRootPath(path)
	if err != nil {
		return nil, err
	}
	f, err := d.root.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errs.ObjectNotFound
		}
		return nil, err
	}
	isFolder := f.IsDir()
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
	fileHandle, openErr := d.root.Open(path)
	if openErr == nil {
		if t, statErr := times.StatFile(fileHandle); statErr == nil && t.HasBirthTime() {
			ctime = t.BirthTime()
		}
		_ = fileHandle.Close()
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
	fullPath, err := cleanRootPath(file.GetPath())
	if err != nil {
		return nil, err
	}
	link := &model.Link{}
	open, err := d.root.Open(fullPath)
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
	parent, err := cleanRootPath(parentDir.GetPath())
	if err != nil {
		return err
	}
	fullPath := filepath.Join(parent, dirName)
	err = d.root.MkdirAll(fullPath, os.FileMode(d.mkdirPerm))
	if err != nil {
		return err
	}
	d.refreshDirectory(parent)
	return nil
}

func (d *Local) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	return d.MoveTo(ctx, srcObj, dstDir, srcObj.GetName())
}

func (d *Local) MoveTo(ctx context.Context, srcObj, dstDir model.Obj, dstName string) error {
	srcPath, err := cleanRootPath(srcObj.GetPath())
	if err != nil {
		return err
	}
	dstDirPath, err := cleanRootPath(dstDir.GetPath())
	if err != nil {
		return err
	}
	dstPath := filepath.Join(dstDirPath, dstName)
	if utils.IsSubPath(srcPath, dstPath) {
		return fmt.Errorf("the destination folder is a subfolder of the source folder")
	}
	err = d.root.Rename(srcPath, dstPath)
	if isCrossDeviceError(err) {
		// 跨设备移动，变更为移动任务
		return errs.NotImplement
	}
	if err == nil {
		srcParent := filepath.Dir(srcPath)
		dstParent := filepath.Dir(dstPath)
		d.refreshDirectory(srcParent)
		d.refreshDirectory(dstParent)
	}
	return err
}

func (d *Local) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	srcPath, err := cleanRootPath(srcObj.GetPath())
	if err != nil {
		return err
	}
	dstPath := filepath.Join(filepath.Dir(srcPath), newName)
	err = d.root.Rename(srcPath, dstPath)
	if err != nil {
		return err
	}

	if srcObj.IsDir() {
		d.refreshDirectory(filepath.Dir(srcPath))
	}

	return nil
}

func (d *Local) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	return d.CopyTo(ctx, srcObj, dstDir, srcObj.GetName())
}

func (d *Local) CopyTo(_ context.Context, srcObj, dstDir model.Obj, dstName string) error {
	srcPath, err := cleanRootPath(srcObj.GetPath())
	if err != nil {
		return err
	}
	dstDirPath, err := cleanRootPath(dstDir.GetPath())
	if err != nil {
		return err
	}
	dstPath := filepath.Join(dstDirPath, dstName)
	if utils.IsSubPath(srcPath, dstPath) {
		return fmt.Errorf("the destination folder is a subfolder of the source folder")
	}
	info, err := d.root.Lstat(srcPath)
	if err != nil {
		return err
	}
	// 复制regular文件会返回errs.NotImplement, 转为复制任务
	if err = d.tryCopy(srcPath, dstPath, info); err != nil {
		return err
	}

	d.refreshDirectory(filepath.Dir(dstPath))

	return nil
}

func (d *Local) Remove(ctx context.Context, obj model.Obj) error {
	objPath, pathErr := cleanRootPath(obj.GetPath())
	if pathErr != nil {
		return pathErr
	}
	var err error
	if utils.SliceContains([]string{"", "delete permanently"}, d.RecycleBinPath) {
		if obj.IsDir() {
			err = d.root.RemoveAll(objPath)
		} else {
			err = d.root.Remove(objPath)
		}
	} else {
		objName := obj.GetName()
		var relPath string
		relPath, err = filepath.Rel(".", filepath.Dir(objPath))
		if err != nil {
			return err
		}
		if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
			return errs.PermissionDenied
		}
		recycleBinPath := filepath.Join(d.recycleBinPath, relPath)
		if err = d.root.MkdirAll(recycleBinPath, 0o755); err != nil {
			return err
		}

		dstPath := filepath.Join(recycleBinPath, objName)
		if _, statErr := d.root.Lstat(dstPath); statErr == nil {
			dstPath = filepath.Join(recycleBinPath, objName+"_"+time.Now().Format("20060102150405"))
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		err = d.root.Rename(objPath, dstPath)
	}
	if err != nil {
		return err
	}
	d.refreshDirectory(filepath.Dir(objPath))

	return nil
}

func (d *Local) Purge(ctx context.Context, obj model.Obj) error {
	objPath, err := cleanRootPath(obj.GetPath())
	if err != nil {
		return err
	}
	if obj.IsDir() {
		err = d.root.RemoveAll(objPath)
	} else {
		err = d.root.Remove(objPath)
	}
	if err != nil {
		return err
	}
	d.refreshDirectory(filepath.Dir(objPath))
	return nil
}

func (d *Local) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	dstDirPath, err := cleanRootPath(dstDir.GetPath())
	if err != nil {
		return err
	}
	fullPath := filepath.Join(dstDirPath, stream.GetName())
	var existingMode fs.FileMode
	existed := false
	if info, statErr := d.root.Stat(fullPath); statErr == nil {
		existed = true
		existingMode = info.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	out, tempPath, err := createUploadTemp(d.root, dstDirPath)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = out.Close()
		if !committed {
			_ = d.root.Remove(tempPath)
		}
	}()
	err = utils.CopyWithCtx(ctx, out, stream, stream.GetSize(), up)
	if err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if existed {
		if err = d.root.Chmod(tempPath, existingMode); err != nil {
			return err
		}
	}
	err = d.root.Chtimes(tempPath, stream.ModTime(), stream.ModTime())
	if err != nil {
		log.Errorf("[local] failed to change time of %s: %s", tempPath, err)
	}
	if err = d.root.Rename(tempPath, fullPath); err != nil {
		return err
	}
	committed = true
	d.refreshDirectory(dstDirPath)

	return nil
}

func (d *Local) refreshDirectory(path string) {
	if !d.DirectorySize {
		return
	}
	if err := d.directoryMap.Refresh(path); err != nil {
		d.directoryMap.MarkDirty()
		log.Warnf("[local] directory size cache is dirty after refreshing %s: %v", path, err)
	}
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
