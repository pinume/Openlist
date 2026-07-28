package local

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/KarpelesLab/reflink"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
)

func cleanRootPath(name string) (string, error) {
	name = filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(name) {
		name = strings.TrimPrefix(name, filepath.VolumeName(name))
		name = strings.TrimLeft(name, `/\`)
	}
	name = filepath.Clean(name)
	if name == "." {
		return name, nil
	}
	if name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(name) {
		return "", errs.PermissionDenied
	}
	return name, nil
}

func createUploadTemp(root *os.Root, dir string) (*os.File, string, error) {
	var random [8]byte
	for range 10 {
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := filepath.Join(dir, ".tinylist-upload-"+hex.EncodeToString(random[:]))
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("failed to allocate a temporary upload file")
}

func isSymlinkDir(root *os.Root, f fs.FileInfo, parent string) bool {
	if f.Mode()&os.ModeSymlink == os.ModeSymlink {
		stat, err := root.Stat(filepath.Join(parent, f.Name()))
		if err != nil {
			return false
		}
		return stat.IsDir()
	}
	return false
}

func readDir(root *os.Root, dirname string) ([]fs.FileInfo, error) {
	f, err := root.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

type DirectoryMap struct {
	root   string
	fsRoot *os.Root
	mu     sync.RWMutex
	data   map[string]*DirectoryNode
	dirty  bool
}

type DirectoryNode struct {
	fileSum      int64
	directorySum int64
	children     []string
}

type DirectoryTask struct {
	path      string
	cache     *DirectoryTaskCache
	ancestors []fs.FileInfo
}

type DirectoryTaskCache struct {
	fileSum  int64
	children []string
}

func (m *DirectoryMap) Configure(root string, fsRoot *os.Root) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.root = root
	m.fsRoot = fsRoot
	m.data = make(map[string]*DirectoryNode)
	m.dirty = false
}

func (m *DirectoryMap) Get(path string) (*DirectoryNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	node, ok := m.data[path]
	if !ok {
		return &DirectoryNode{}, false
	}
	copyNode := *node
	copyNode.children = append([]string(nil), node.children...)
	return &copyNode, true
}

func (m *DirectoryMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearLocked()
}

func (m *DirectoryMap) MarkDirty() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirty = true
}

func (m *DirectoryMap) RecalculateDirSize() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recalculateDirSizeLocked()
}

func (m *DirectoryMap) recalculateDirSizeLocked() error {
	m.clearLocked()
	if m.root == "" {
		return fmt.Errorf("root path is not set")
	}
	if m.fsRoot == nil {
		return fmt.Errorf("root filesystem is not set")
	}
	if _, err := m.calculateDirSizeLocked(m.root); err != nil {
		m.dirty = true
		return err
	}
	m.dirty = false
	return nil
}

func (m *DirectoryMap) calculateDirSizeLocked(dirname string) (int64, error) {
	rootInfo, err := m.fsRoot.Stat(dirname)
	if err != nil {
		return 0, err
	}
	stack := []DirectoryTask{
		{path: dirname, ancestors: []fs.FileInfo{rootInfo}},
	}

	for len(stack) > 0 {
		task := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if task.cache != nil {
			directorySum := int64(0)

			for _, filename := range task.cache.children {
				child, ok := m.getLocked(filepath.Join(task.path, filename))
				if !ok {
					return 0, fmt.Errorf("child node not found")
				}
				directorySum += child.fileSum + child.directorySum
			}

			m.setLocked(task.path, &DirectoryNode{
				fileSum:      task.cache.fileSum,
				directorySum: directorySum,
				children:     task.cache.children,
			})

			continue
		}

		files, err := readDir(m.fsRoot, task.path)
		if err != nil {
			return 0, err
		}

		fileSum := int64(0)
		directorySum := int64(0)

		children := []string{}
		queue := []DirectoryTask{}

		for _, f := range files {
			fullpath := filepath.Join(task.path, f.Name())
			isFolder := f.IsDir() || isSymlinkDir(m.fsRoot, f, task.path)

			if isFolder {
				if node, ok := m.getLocked(fullpath); ok {
					directorySum += node.fileSum + node.directorySum
				} else {
					dirInfo := f
					if f.Mode()&os.ModeSymlink != 0 {
						dirInfo, err = m.fsRoot.Stat(fullpath)
						if err != nil {
							return 0, err
						}
					}
					if sameFileIn(dirInfo, task.ancestors) {
						m.setLocked(fullpath, &DirectoryNode{})
					} else {
						ancestors := append([]fs.FileInfo(nil), task.ancestors...)
						ancestors = append(ancestors, dirInfo)
						queue = append(queue, DirectoryTask{
							path:      fullpath,
							ancestors: ancestors,
						})
					}
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

		m.setLocked(task.path, &DirectoryNode{
			fileSum:      fileSum,
			directorySum: directorySum,
			children:     children,
		})
	}

	if node, ok := m.getLocked(dirname); ok {
		return node.fileSum + node.directorySum, nil
	}

	return 0, nil
}

func sameFileIn(info fs.FileInfo, candidates []fs.FileInfo) bool {
	for _, candidate := range candidates {
		if os.SameFile(info, candidate) {
			return true
		}
	}
	return false
}

func (m *DirectoryMap) updateDirSizeLocked(dirname string) (int64, error) {
	node, ok := m.getLocked(dirname)
	if !ok {
		return 0, fmt.Errorf("directory node not found")
	}

	files, err := readDir(m.fsRoot, dirname)
	if err != nil {
		return 0, err
	}
	fileSum := int64(0)
	directorySum := int64(0)

	children := []string{}

	for _, f := range files {
		fullpath := filepath.Join(dirname, f.Name())
		isFolder := f.IsDir() || isSymlinkDir(m.fsRoot, f, dirname)

		if isFolder {
			if node, ok := m.getLocked(fullpath); ok {
				directorySum += node.fileSum + node.directorySum
			} else {
				value, err := m.calculateDirSizeLocked(fullpath)
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

	currentChildren := make(map[string]struct{}, len(children))
	for _, child := range children {
		currentChildren[child] = struct{}{}
	}
	for _, child := range node.children {
		if _, ok := currentChildren[child]; !ok {
			m.deleteDirNodeLocked(filepath.Join(dirname, child))
		}
	}

	node.fileSum = fileSum
	node.directorySum = directorySum
	node.children = children

	return fileSum + directorySum, nil
}

func (m *DirectoryMap) deleteDirNodeLocked(dirname string) {
	stack := []string{dirname}

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node, ok := m.getLocked(current); ok {
			for _, filename := range node.children {
				stack = append(stack, filepath.Join(current, filename))
			}

			delete(m.data, current)
		}
	}
}

func (m *DirectoryMap) Refresh(dirname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dirty {
		return m.recalculateDirSizeLocked()
	}
	if !m.containsLocked(dirname) {
		m.dirty = true
		return m.recalculateDirSizeLocked()
	}
	if _, ok := m.getLocked(dirname); !ok {
		if _, err := m.calculateDirSizeLocked(dirname); err != nil {
			m.dirty = true
			return m.recalculateDirSizeLocked()
		}
	} else {
		m.deleteDirNodeLocked(dirname)
		if _, err := m.calculateDirSizeLocked(dirname); err != nil {
			m.dirty = true
			return m.recalculateDirSizeLocked()
		}
	}
	current := filepath.Dir(dirname)
	for m.containsLocked(current) {
		if _, ok := m.getLocked(current); !ok {
			m.dirty = true
			return m.recalculateDirSizeLocked()
		}
		if _, err := m.updateDirSizeLocked(current); err != nil {
			m.dirty = true
			return m.recalculateDirSizeLocked()
		}
		if current == m.root {
			break
		}
		current = filepath.Dir(current)
	}
	m.dirty = false
	return nil
}

func (m *DirectoryMap) getLocked(path string) (*DirectoryNode, bool) {
	node, ok := m.data[path]
	return node, ok
}

func (m *DirectoryMap) setLocked(path string, node *DirectoryNode) {
	if m.data == nil {
		m.data = make(map[string]*DirectoryNode)
	}
	m.data[path] = node
}

func (m *DirectoryMap) clearLocked() {
	m.data = make(map[string]*DirectoryNode)
}

func (m *DirectoryMap) containsLocked(path string) bool {
	rel, err := filepath.Rel(m.root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func (d *Local) tryCopy(srcPath, dstPath string, info os.FileInfo) error {
	if info.Mode()&os.ModeDevice != 0 {
		return errors.New("cannot copy a device")
	} else if info.Mode()&os.ModeSymlink != 0 {
		return d.copySymlink(srcPath, dstPath)
	} else if info.Mode()&os.ModeNamedPipe != 0 {
		return d.copyNamedPipe(dstPath, info.Mode())
	} else if info.IsDir() {
		return d.recurAndTryCopy(srcPath, dstPath)
	} else {
		return d.tryReflinkCopy(srcPath, dstPath, info.Mode())
	}
}

func (d *Local) copySymlink(srcPath, dstPath string) error {
	linkOrig, err := d.root.Readlink(srcPath)
	if err != nil {
		return err
	}
	dstDir := filepath.Dir(dstPath)
	if !filepath.IsAbs(linkOrig) {
		srcDir := filepath.Dir(srcPath)
		rel, err := filepath.Rel(dstDir, srcDir)
		if err != nil {
			return err
		}
		linkOrig = filepath.Clean(filepath.Join(rel, linkOrig))
	}
	err = d.root.MkdirAll(dstDir, os.FileMode(d.mkdirPerm))
	if err != nil {
		return err
	}
	return d.root.Symlink(linkOrig, dstPath)
}

func (d *Local) recurAndTryCopy(srcPath, dstPath string) error {
	err := d.root.MkdirAll(dstPath, os.FileMode(d.mkdirPerm))
	if err != nil {
		return err
	}
	files, err := readDir(d.root, srcPath)
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

func (d *Local) tryReflinkCopy(srcPath, dstPath string, mode os.FileMode) error {
	src, err := d.root.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := d.root.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	defer dst.Close()
	err = reflink.Reflink(dst, src, false)
	if errors.Is(err, reflink.ErrReflinkUnsupported) || errors.Is(err, reflink.ErrReflinkFailed) || isCrossDeviceError(err) {
		_ = d.root.Remove(dstPath)
		return errs.NotImplement
	}
	if err != nil {
		_ = d.root.Remove(dstPath)
	}
	return err
}
