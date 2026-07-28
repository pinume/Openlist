// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package webdav

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"path"
	"path/filepath"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/fs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/pkg/errors"
)

// slashClean is equivalent to but slightly more efficient than
// path.Clean("/" + name).
func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return path.Clean(name)
}

// moveFiles moves files and/or directories from src to dst.
// Individual item permission checks are skipped for performance reasons.
//
// See section 9.9.4 for when various HTTP status codes apply.
func moveFiles(ctx context.Context, src, dst string, overwrite bool) (status int, err error) {
	srcDir := path.Dir(src)
	dstDir := path.Dir(dst)
	srcName := path.Base(src)
	dstName := path.Base(dst)
	if slashClean(src) == slashClean(dst) {
		return http.StatusForbidden, nil
	}
	user, ok := common.UserFromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, nil
	}
	if srcDir != dstDir && !user.CanMove() {
		return http.StatusForbidden, nil
	}
	if srcName != dstName && !user.CanRename() {
		return http.StatusForbidden, nil
	}
	srcMeta, err := op.GetNearestMeta(srcDir)
	if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
		return http.StatusInternalServerError, err
	}
	dstMeta, err := op.GetNearestMeta(dstDir)
	if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
		return http.StatusInternalServerError, err
	}
	if !common.CanWrite(user, srcMeta, srcDir) || !common.CanWrite(user, dstMeta, dstDir) {
		return http.StatusForbidden, nil
	}
	srcAllowed, err := common.CanWriteTree(user, src)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	dstAllowed, err := common.CanWriteTree(user, dst)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !srcAllowed || !dstAllowed {
		return http.StatusForbidden, nil
	}
	existed, backupPath, status, err := prepareDestination(ctx, dst, overwrite)
	if status != 0 || err != nil {
		return status, err
	}
	_, err = fs.MoveTo(context.WithValue(ctx, conf.NoTaskKey, struct{}{}), src, dst)
	if err != nil {
		return http.StatusInternalServerError, compensateFailedTransfer(ctx, dst, backupPath, err)
	}
	if cleanupErr := cleanupBackup(ctx, backupPath); cleanupErr != nil {
		return http.StatusInternalServerError, errors.Wrapf(cleanupErr, "move completed at %s but old destination remains at %s", dst, backupPath)
	}
	if existed {
		return http.StatusNoContent, nil
	}
	return http.StatusCreated, nil
}

// copyFiles copies files and/or directories from src to dst.
// Individual item permission checks are skipped for performance reasons.
//
// See section 9.8.5 for when various HTTP status codes apply.
func copyFiles(ctx context.Context, src, dst string, overwrite bool) (status int, err error) {
	srcDir := path.Dir(src)
	dstDir := path.Dir(dst)
	if slashClean(src) == slashClean(dst) {
		return http.StatusForbidden, nil
	}
	user, ok := common.UserFromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, nil
	}
	if !user.CanCopy() {
		return http.StatusForbidden, nil
	}
	srcMeta, err := op.GetNearestMeta(srcDir)
	if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
		return http.StatusInternalServerError, err
	}
	if !common.CanRead(user, srcMeta, srcDir) {
		return http.StatusForbidden, nil
	}
	dstMeta, err := op.GetNearestMeta(dstDir)
	if err != nil && !errors.Is(errors.Cause(err), errs.MetaNotFound) {
		return http.StatusInternalServerError, err
	}
	if !common.CanWrite(user, dstMeta, dstDir) {
		return http.StatusForbidden, nil
	}
	srcAllowed, err := common.CanReadTree(user, src)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	dstAllowed, err := common.CanWriteTree(user, dst)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !srcAllowed || !dstAllowed {
		return http.StatusForbidden, nil
	}
	existed, backupPath, status, err := prepareDestination(ctx, dst, overwrite)
	if status != 0 || err != nil {
		return status, err
	}
	_, err = fs.CopyTo(context.WithValue(ctx, conf.NoTaskKey, struct{}{}), src, dst)
	if err != nil {
		return http.StatusInternalServerError, compensateFailedTransfer(ctx, dst, backupPath, err)
	}
	if cleanupErr := cleanupBackup(ctx, backupPath); cleanupErr != nil {
		return http.StatusInternalServerError, errors.Wrapf(cleanupErr, "copy completed at %s but old destination remains at %s", dst, backupPath)
	}
	if existed {
		return http.StatusNoContent, nil
	}
	return http.StatusCreated, nil
}

func prepareDestination(ctx context.Context, dst string, overwrite bool) (existed bool, backupPath string, status int, err error) {
	_, err = fs.Get(ctx, dst, &fs.GetArgs{NoLog: true})
	if err != nil {
		if errs.IsObjectNotFound(err) {
			return false, "", 0, nil
		}
		return false, "", http.StatusInternalServerError, err
	}
	if !overwrite {
		return true, "", http.StatusPreconditionFailed, nil
	}
	backupPath, err = unusedBackupPath(ctx, dst)
	if err != nil {
		return true, "", http.StatusInternalServerError, err
	}
	if err = fs.Rename(ctx, dst, path.Base(backupPath)); err != nil {
		return true, "", http.StatusInternalServerError, errors.Wrapf(err, "failed to preserve existing destination %s", dst)
	}
	return true, backupPath, 0, nil
}

func unusedBackupPath(ctx context.Context, dst string) (string, error) {
	var randomBytes [8]byte
	for range 10 {
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return "", err
		}
		candidate := path.Join(path.Dir(dst), ".tinylist-webdav-"+hex.EncodeToString(randomBytes[:]))
		if _, err := fs.Get(ctx, candidate, &fs.GetArgs{NoLog: true}); errs.IsObjectNotFound(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("failed to allocate a WebDAV backup path")
}

func compensateFailedTransfer(ctx context.Context, dst, backupPath string, transferErr error) error {
	cleanupErr := removeIfExists(ctx, dst)
	if backupPath == "" {
		if cleanupErr != nil {
			return errors.Wrapf(transferErr, "transfer failed and partial destination remains at %s: cleanup failed: %v", dst, cleanupErr)
		}
		return transferErr
	}
	if cleanupErr != nil {
		return errors.Wrapf(transferErr, "transfer failed; old destination remains at %s and partial destination remains at %s: cleanup failed: %v", backupPath, dst, cleanupErr)
	}
	if err := fs.Rename(ctx, backupPath, path.Base(dst)); err != nil {
		return errors.Wrapf(transferErr, "transfer failed and restoring %s from %s also failed: %v", dst, backupPath, err)
	}
	return transferErr
}

func cleanupBackup(ctx context.Context, backupPath string) error {
	if backupPath == "" {
		return nil
	}
	return fs.Purge(ctx, backupPath)
}

func removeIfExists(ctx context.Context, target string) error {
	_, err := fs.Get(ctx, target, &fs.GetArgs{NoLog: true})
	if errs.IsObjectNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fs.Purge(ctx, target)
}

// walkFS traverses filesystem fs starting at name up to depth levels.
//
// Allowed values for depth are 0, 1 or infiniteDepth. For each visited node,
// walkFS calls walkFn. If a visited file system node is a directory and
// walkFn returns path.SkipDir, walkFS will skip traversal of this node.
func walkFS(ctx context.Context, depth int, name string, info model.Obj, walkFn func(reqPath string, info model.Obj, err error) error) error {
	// This implementation is based on Walk's code in the standard path/path package.
	err := walkFn(name, info, nil)
	if err != nil {
		if info.IsDir() && err == filepath.SkipDir {
			return nil
		}
		return err
	}
	if !info.IsDir() || depth == 0 {
		return nil
	}
	if depth == 1 {
		depth = 0
	}
	meta, _ := op.GetNearestMeta(name)
	// Read directory names.
	objs, err := fs.List(context.WithValue(ctx, conf.MetaKey, meta), name, &fs.ListArgs{})
	//f, err := fs.OpenFile(ctx, name, os.O_RDONLY, 0)
	//if err != nil {
	//	return walkFn(name, info, err)
	//}
	//fileInfos, err := f.Readdir(0)
	//f.Close()
	if err != nil {
		return walkFn(name, info, err)
	}

	for _, fileInfo := range objs {
		filename := path.Join(name, fileInfo.GetName())
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return err
			}
		} else {
			err = walkFS(ctx, depth, filename, fileInfo, walkFn)
			if err != nil {
				if !fileInfo.IsDir() || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}
