package local

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/KarpelesLab/reflink"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
)

func createUploadTemp(dir string) (*os.File, error) {
	var random [8]byte
	for range 10 {
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		name := filepath.Join(dir, ".tinylist-upload-"+hex.EncodeToString(random[:]))
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return nil, errors.New("failed to allocate a temporary upload file")
}

func isSymlinkDir(f fs.FileInfo, path string) bool {
	if f.Mode()&os.ModeSymlink == os.ModeSymlink {
		dst, err := os.Readlink(filepath.Join(path, f.Name()))
		if err != nil {
			return false
		}
		if !filepath.IsAbs(dst) {
			dst = filepath.Join(path, dst)
		}
		stat, err := os.Stat(dst)
		if err != nil {
			return false
		}
		return stat.IsDir()
	}
	return false
}

func readDir(dirname string) ([]fs.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

type DirectoryMap struct {
	root string
	data sync.Map
}

type DirectoryNode struct {
	fileSum      int64
	directorySum int64
	children     []string
}

type DirectoryTask struct {
	path  string
	cache *DirectoryTaskCache
}

type DirectoryTaskCache struct {
	fileSum  int64
	children []string
}

func (m *DirectoryMap) Has(path string) bool {
	_, ok := m.data.Load(path)

	return ok
}

func (m *DirectoryMap) Get(path string) (*DirectoryNode, bool) {
	value, ok := m.data.Load(path)
	if !ok {
		return &DirectoryNode{}, false
	}

	node, ok := value.(*DirectoryNode)
	if !ok {
		return &DirectoryNode{}, false
	}

	return node, true
}

func (m *DirectoryMap) Set(path string, node *DirectoryNode) {
	m.data.Store(path, node)
}

func (m *DirectoryMap) Delete(path string) {
	m.data.Delete(path)
}

func (m *DirectoryMap) Clear() {
	m.data.Clear()
}

func (m *DirectoryMap) RecalculateDirSize() error {
	m.Clear()
	if m.root == "" {
		return fmt.Errorf("root path is not set")
	}

	size, err := m.CalculateDirSize(m.root)
	if err != nil {
		return err
	}

	if node, ok := m.Get(m.root); ok {
		node.fileSum = size
		node.directorySum = size
	}

	return nil
}

func (m *DirectoryMap) CalculateDirSize(dirname string) (int64, error) {
	stack := []DirectoryTask{
		{path: dirname},
	}

	for len(stack) > 0 {
		task := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if task.cache != nil {
			directorySum := int64(0)

			for _, filename := range task.cache.children {
				child, ok := m.Get(filepath.Join(task.path, filename))
				if !ok {
					return 0, fmt.Errorf("child node not found")
				}
				directorySum += child.fileSum + child.directorySum
			}

			m.Set(task.path, &DirectoryNode{
				fileSum:      task.cache.fileSum,
				directorySum: directorySum,
				children:     task.cache.children,
			})

			continue
		}

		files, err := readDir(task.path)
		if err != nil {
			return 0, err
		}

		fileSum := int64(0)
		directorySum := int64(0)

		children := []string{}
		queue := []DirectoryTask{}

		for _, f := range files {
			fullpath := filepath.Join(task.path, f.Name())
			isFolder := f.IsDir() || isSymlinkDir(f, fullpath)

			if isFolder {
				if node, ok := m.Get(fullpath); ok {
					directorySum += node.fileSum + node.directorySum
				} else {
					queue = append(queue, DirectoryTask{
						path: fullpath,
					})
				}

				children = append(children, f.Name())
			} else {
				fileSum += f.Size()
			}
		}

		if len(queue) > 0 {
			stack = append(stack, DirectoryTask{
				path: task.path,
				cache: &DirectoryTaskCache{
					fileSum:  fileSum,
					children: children,
				},
			})

			stack = append(stack, queue...)

			continue
		}

		m.Set(task.path, &DirectoryNode{
			fileSum:      fileSum,
			directorySum: directorySum,
			children:     children,
		})
	}

	if node, ok := m.Get(dirname); ok {
		return node.fileSum + node.directorySum, nil
	}

	return 0, nil
}

func (m *DirectoryMap) UpdateDirSize(dirname string) (int64, error) {
	node, ok := m.Get(dirname)
	if !ok {
		return 0, fmt.Errorf("directory node not found")
	}

	files, err := readDir(dirname)
	if err != nil {
		return 0, err
	}
	fileSum := int64(0)
	directorySum := int64(0)

	children := []string{}

	for _, f := range files {
		fullpath := filepath.Join(dirname, f.Name())
		isFolder := f.IsDir() || isSymlinkDir(f, fullpath)

		if isFolder {
			if node, ok := m.Get(fullpath); ok {
				directorySum += node.fileSum + node.directorySum
			} else {
				value, err := m.CalculateDirSize(fullpath)
				if err != nil {
					return 0, err
				}
				directorySum += value
			}

			children = append(children, f.Name())
		} else {
			fileSum += f.Size()
		}
	}

	for _, c := range node.children {
		if !slices.Contains(children, c) {
			m.DeleteDirNode(filepath.Join(dirname, c))
		}
	}

	node.fileSum = fileSum
	node.directorySum = directorySum
	node.children = children

	return fileSum + directorySum, nil
}

func (m *DirectoryMap) UpdateDirParents(dirname string) error {
	parentPath := filepath.Dir(dirname)
	for parentPath != m.root && !strings.HasPrefix(m.root, parentPath) {
		if node, ok := m.Get(parentPath); ok {
			directorySum := int64(0)

			for _, c := range node.children {
				child, ok := m.Get(filepath.Join(parentPath, c))
				if !ok {
					return fmt.Errorf("child node not found")
				}
				directorySum += child.fileSum + child.directorySum
			}

			node.directorySum = directorySum
		}

		parentPath = filepath.Dir(parentPath)
	}

	return nil
}

func (m *DirectoryMap) DeleteDirNode(dirname string) error {
	stack := []string{dirname}

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node, ok := m.Get(current); ok {
			for _, filename := range node.children {
				stack = append(stack, filepath.Join(current, filename))
			}

			m.Delete(current)
		}
	}

	return nil
}

func (d *Local) tryCopy(srcPath, dstPath string, info os.FileInfo) error {
	if info.Mode()&os.ModeDevice != 0 {
		return errors.New("cannot copy a device")
	} else if info.Mode()&os.ModeSymlink != 0 {
		return d.copySymlink(srcPath, dstPath)
	} else if info.Mode()&os.ModeNamedPipe != 0 {
		return copyNamedPipe(dstPath, info.Mode(), os.FileMode(d.mkdirPerm))
	} else if info.IsDir() {
		return d.recurAndTryCopy(srcPath, dstPath)
	} else {
		return tryReflinkCopy(srcPath, dstPath)
	}
}

func (d *Local) copySymlink(srcPath, dstPath string) error {
	linkOrig, err := os.Readlink(srcPath)
	if err != nil {
		return err
	}
	dstDir := filepath.Dir(dstPath)
	if !filepath.IsAbs(linkOrig) {
		srcDir := filepath.Dir(srcPath)
		rel, err := filepath.Rel(dstDir, srcDir)
		if err != nil {
			rel, err = filepath.Abs(srcDir)
		}
		if err != nil {
			return err
		}
		linkOrig = filepath.Clean(filepath.Join(rel, linkOrig))
	}
	err = os.MkdirAll(dstDir, os.FileMode(d.mkdirPerm))
	if err != nil {
		return err
	}
	return os.Symlink(linkOrig, dstPath)
}

func (d *Local) recurAndTryCopy(srcPath, dstPath string) error {
	err := os.MkdirAll(dstPath, os.FileMode(d.mkdirPerm))
	if err != nil {
		return err
	}
	files, err := readDir(srcPath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() {
			sp := filepath.Join(srcPath, f.Name())
			dp := filepath.Join(dstPath, f.Name())
			if err = d.tryCopy(sp, dp, f); err != nil {
				return err
			}
		}
	}
	for _, f := range files {
		if f.IsDir() {
			sp := filepath.Join(srcPath, f.Name())
			dp := filepath.Join(dstPath, f.Name())
			if err = d.recurAndTryCopy(sp, dp); err != nil {
				return err
			}
		}
	}
	return nil
}

func tryReflinkCopy(srcPath, dstPath string) error {
	err := reflink.Always(srcPath, dstPath)
	if errors.Is(err, reflink.ErrReflinkUnsupported) || errors.Is(err, reflink.ErrReflinkFailed) || isCrossDeviceError(err) {
		return errs.NotImplement
	}
	return err
}
