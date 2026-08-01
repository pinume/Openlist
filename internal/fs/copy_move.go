package fs

import (
	"context"
	"fmt"
	stdpath "path"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/internal/task"
	"github.com/OpenListTeam/OpenList/v4/internal/task_group"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/OpenListTeam/tache"
	"github.com/pkg/errors"
)

type taskType uint8

func (t taskType) String() string {
	switch t {
	case copy:
		return "copy"
	case move:
		return "move"
	case merge:
		return "merge"
	default:
		return "unknown"
	}
}

const (
	copy taskType = iota
	move
	merge
)

type FileTransferTask struct {
	TaskData
	TaskType taskType
	groupID  string
}

func (t *FileTransferTask) GetName() string {
	return fmt.Sprintf("%s [%s](%s) to [%s](%s)", t.TaskType, t.SrcStorageMp, t.SrcActualPath, t.DstStorageMp, t.DstActualPath)
}

func (t *FileTransferTask) Run() error {
	if t.SrcStorage == nil {
		if srcStorage, _, err := op.GetStorageAndActualPath(t.SrcStorageMp); err == nil {
			t.SrcStorage = srcStorage
		} else {
			return err
		}
		if dstStorage, _, err := op.GetStorageAndActualPath(t.DstStorageMp); err == nil {
			t.DstStorage = dstStorage
		} else {
			return err
		}
	}

	t.ClearEndTime()
	t.SetStartTime(time.Now())
	defer func() { t.SetEndTime(time.Now()) }()
	return t.RunWithNextTaskCallback(func(nextTask *FileTransferTask) error {
		task_group.TransferCoordinator.AddTask(t.groupID, nil)
		if t.TaskType == copy || t.TaskType == merge {
			CopyTaskManager.Add(nextTask)
		} else {
			MoveTaskManager.Add(nextTask)
		}
		return nil
	})
}

func (t *FileTransferTask) OnSucceeded() {
	task_group.TransferCoordinator.Done(context.WithoutCancel(t.Ctx()), t.groupID, true)
}

func (t *FileTransferTask) OnFailed() {
	task_group.TransferCoordinator.Done(context.WithoutCancel(t.Ctx()), t.groupID, false)
}

func (t *FileTransferTask) SetRetry(retry int, maxRetry int) {
	t.TaskData.SetRetry(retry, maxRetry)
	if retry == 0 &&
		(len(t.groupID) == 0 || // 重启恢复
			(t.GetErr() == nil && t.GetState() != tache.StatePending)) { // 手动重试
		t.groupID = stdpath.Join(t.DstStorageMp, t.DstActualPath, t.DstName)
		var payload any
		if t.TaskType == move {
			payload = task_group.SrcPathToRemove(stdpath.Join(t.SrcStorageMp, t.SrcActualPath))
		}
		task_group.TransferCoordinator.AddTask(t.groupID, payload)
	}
}

func transfer(ctx context.Context, taskType taskType, srcObjPath, dstDirPath, dstName string, skipHook ...bool) (task.TaskExtensionInfo, error) {
	skipExisting := taskType == copy && ctx.Value(conf.SkipExistingKey) != nil
	srcStorage, srcObjActualPath, err := op.GetStorageAndActualPath(srcObjPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed get src storage")
	}
	dstStorage, dstDirActualPath, err := op.GetStorageAndActualPath(dstDirPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed get dst storage")
	}

	if srcStorage.GetStorage() == dstStorage.GetStorage() {
		// Check the size match before touching the driver's native Copy,
		// which would otherwise open the source (blocking on a FIFO,
		// dereferencing a symlink instead of copying it, or losing
		// reflink/special-file handling) even when the file is about to
		// be skipped. This only ever fires for a plain file: a directory
		// source always falls through to the native Copy below, same as
		// before skip_existing existed.
		if skipExisting {
			if srcObj, srcErr := op.Get(ctx, srcStorage, srcObjActualPath); srcErr == nil {
				checkDstName := dstName
				if checkDstName == "" {
					checkDstName = srcObj.GetName()
				}
				if dstObj, dstErr := op.Get(ctx, dstStorage, stdpath.Join(dstDirActualPath, checkDstName)); dstErr == nil &&
					shouldSkipExistingFile(srcObj, dstObj) {
					return nil, nil
				}
			}
		}
		if utils.IsBool(skipHook...) {
			ctx = context.WithValue(ctx, conf.SkipHookKey, struct{}{})
		}
		if taskType == copy || taskType == merge {
			err = op.Copy(ctx, srcStorage, srcObjActualPath, dstDirActualPath, dstName)
			if !errors.Is(err, errs.NotImplement) && !errors.Is(err, errs.NotSupport) {
				return nil, err
			}
		} else {
			err = op.Move(ctx, srcStorage, srcObjActualPath, dstDirActualPath, dstName)
			if !errors.Is(err, errs.NotImplement) && !errors.Is(err, errs.NotSupport) {
				return nil, err
			}
		}
	}

	// not in the same storage
	t := &FileTransferTask{
		TaskData: TaskData{
			SrcStorage:    srcStorage,
			DstStorage:    dstStorage,
			SrcActualPath: srcObjActualPath,
			DstActualPath: dstDirActualPath,
			DstName:       dstName,
			SrcStorageMp:  srcStorage.GetStorage().MountPath,
			DstStorageMp:  dstStorage.GetStorage().MountPath,
			SkipExisting:  skipExisting,
		},
		TaskType: taskType,
	}

	t.groupID = stdpath.Join(t.DstStorageMp, t.DstActualPath, t.DstName)
	task_group.TransferCoordinator.AddTask(t.groupID, nil)
	if ctx.Value(conf.NoTaskKey) != nil {
		var callback func(nextTask *FileTransferTask) error
		hasSuccess := false
		callback = func(nextTask *FileTransferTask) error {
			nextTask.Base.SetCtx(ctx)
			err := nextTask.RunWithNextTaskCallback(callback)
			if err == nil {
				hasSuccess = true
			}
			return err
		}
		t.Base.SetCtx(ctx)
		err = t.RunWithNextTaskCallback(callback)
		if err == nil {
			hasSuccess = true
		}
		if taskType == move {
			task_group.TransferCoordinator.AppendPayload(t.groupID, task_group.SrcPathToRemove(srcObjPath))
		}
		task_group.TransferCoordinator.Done(context.WithoutCancel(ctx), t.groupID, hasSuccess)
		return nil, err
	}

	t.Creator, _ = ctx.Value(conf.UserKey).(*model.User)
	t.ApiUrl = common.GetApiUrl(ctx)
	if taskType == copy || taskType == merge {
		CopyTaskManager.Add(t)
	} else {
		task_group.TransferCoordinator.AppendPayload(t.groupID, task_group.SrcPathToRemove(srcObjPath))
		MoveTaskManager.Add(t)
	}
	return t, nil
}

func (t *FileTransferTask) RunWithNextTaskCallback(f func(nextTask *FileTransferTask) error) error {
	t.Status = "getting src object"
	srcObj, err := op.Get(t.Ctx(), t.SrcStorage, t.SrcActualPath)
	if err != nil {
		return errors.WithMessagef(err, "failed get src [%s] file", t.SrcActualPath)
	}

	if srcObj.IsDir() {
		t.Status = "src object is dir, listing objs"
		objs, err := op.List(t.Ctx(), t.SrcStorage, t.SrcActualPath, model.ListArgs{})
		if err != nil {
			return errors.WithMessagef(err, "failed list src [%s] objs", t.SrcActualPath)
		}
		dstName := srcObj.GetName()
		if t.DstName != "" {
			dstName = t.DstName
		}
		dstActualPath := stdpath.Join(t.DstActualPath, dstName)
		task_group.TransferCoordinator.AppendPayload(t.groupID, task_group.DstPathToHook(dstActualPath))

		existedObjs := make(map[string]bool)
		if t.TaskType == merge {
			dstObjs, err := op.List(t.Ctx(), t.DstStorage, dstActualPath, model.ListArgs{})
			if err != nil && !errors.Is(err, errs.ObjectNotFound) {
				// 目标文件夹不存在的情况不是错误，会在之后新建文件夹
				// 这种情况显然不需要统计existedObjs，dstObjs保持为nil，下面这个for将不会执行
				return errors.WithMessagef(err, "failed list dst [%s] objs", dstActualPath)
			}
			for _, obj := range dstObjs {
				if err := t.Ctx().Err(); err != nil {
					return err
				}
				if !obj.IsDir() {
					existedObjs[obj.GetName()] = true
				}
			}
		}

		for _, obj := range objs {
			if err := t.Ctx().Err(); err != nil {
				return err
			}

			if t.TaskType == merge && !obj.IsDir() && existedObjs[obj.GetName()] {
				// skip existed file
				continue
			}

			err = f(&FileTransferTask{
				TaskType: t.TaskType,
				TaskData: TaskData{
					TaskExtension: task.TaskExtension{
						Creator: t.Creator,
						ApiUrl:  t.ApiUrl,
					},
					SrcStorage:    t.SrcStorage,
					DstStorage:    t.DstStorage,
					SrcActualPath: stdpath.Join(t.SrcActualPath, obj.GetName()),
					DstActualPath: dstActualPath,
					DstName:       "",
					SrcStorageMp:  t.SrcStorageMp,
					DstStorageMp:  t.DstStorageMp,
					SkipExisting:  t.SkipExisting,
				},
				groupID: t.groupID,
			})
			if err != nil {
				return err
			}
		}
		t.Status = fmt.Sprintf("src object is dir, added all %s tasks of objs", t.TaskType)
		return nil
	}

	dstName := srcObj.GetName()
	if t.DstName != "" {
		dstName = t.DstName
	}
	// Check before requesting a link: op.Link can open a source file (or,
	// for a remote driver, request a download URL), which must not happen
	// for a file that turns out to be skipped.
	if t.TaskType == copy && t.SkipExisting {
		dstObj, dstErr := op.Get(t.Ctx(), t.DstStorage, stdpath.Join(t.DstActualPath, dstName))
		if dstErr == nil && shouldSkipExistingFile(srcObj, dstObj) {
			t.SetTotalBytes(srcObj.GetSize())
			t.SetProgress(100)
			t.Status = "skipped: destination file already exists with the same size"
			return nil
		}
	}

	t.Status = "getting src object link"
	link, srcObj, err := op.Link(t.Ctx(), t.SrcStorage, t.SrcActualPath, model.LinkArgs{})
	if err != nil {
		return errors.WithMessagef(err, "failed get [%s] link", t.SrcActualPath)
	}
	// any link provided is seekable
	streamObj := srcObj
	if t.DstName != "" {
		streamObj = &model.ObjWrapName{Name: t.DstName, Obj: srcObj}
	}
	ss, err := stream.NewSeekableStream(&stream.FileStream{
		Obj: streamObj,
		Ctx: t.Ctx(),
	}, link)
	if err != nil {
		_ = link.Close()
		return errors.WithMessagef(err, "failed get [%s] stream", t.SrcActualPath)
	}
	t.SetTotalBytes(ss.GetSize())
	t.Status = "uploading"
	return op.Put(context.WithValue(t.Ctx(), conf.SkipHookKey, struct{}{}), t.DstStorage, t.DstActualPath, ss, t.SetProgress)
}

// shouldSkipExistingFile reports whether a copy of src onto dst should be
// skipped instead of overwritten. It only ever applies to two plain files
// with the same size: a directory on either side, or a size mismatch,
// always leads to a (re-)copy instead of a silent skip, since neither a
// directory's total size nor a partially-copied file reliably means the
// contents already match.
func shouldSkipExistingFile(src, dst model.Obj) bool {
	return !src.IsDir() && !dst.IsDir() &&
		src.GetSize() >= 0 && src.GetSize() == dst.GetSize()
}

var (
	CopyTaskManager *tache.Manager[*FileTransferTask]
	MoveTaskManager *tache.Manager[*FileTransferTask]
)
