package op

import (
	"context"
	stdpath "path"
	"strconv"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/archive/tool"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

func GetArchiveToolAndStream(ctx context.Context, storage driver.Driver, path string, args model.LinkArgs) (model.Obj, tool.Tool, []*stream.SeekableStream, error) {
	l, obj, err := Link(ctx, storage, path, args)
	if err != nil {
		return nil, nil, nil, errors.WithMessagef(err, "failed get [%s] link", path)
	}

	// Get archive tool
	var partExt *tool.MultipartExtension
	var t tool.Tool
	ext := obj.GetName()
	for {
		var found bool
		_, ext, found = strings.Cut(ext, ".")
		if !found {
			_ = l.Close()
			return nil, nil, nil, errors.Errorf("failed get archive tool: the obj does not have an extension.")
		}
		partExt, t, err = tool.GetArchiveTool("." + ext)
		if err == nil {
			break
		}
	}

	// Get first part stream
	ss, err := stream.NewSeekableStream(&stream.FileStream{Ctx: ctx, Obj: obj}, l)
	if err != nil {
		_ = l.Close()
		return nil, nil, nil, errors.WithMessagef(err, "failed get [%s] stream", path)
	}
	ret := []*stream.SeekableStream{ss}
	if partExt == nil {
		return obj, t, ret, nil
	}

	// Merge multi-part archive
	dir := stdpath.Dir(path)
	objs, err := List(ctx, storage, dir, model.ListArgs{})
	if err != nil {
		return obj, t, ret, nil
	}
	for _, o := range objs {
		submatch := partExt.PartFileFormat.FindStringSubmatch(o.GetName())
		if submatch == nil {
			continue
		}
		partIdx, e := strconv.Atoi(submatch[1])
		if e != nil {
			continue
		}
		partIdx = partIdx - partExt.SecondPartIndex + 1
		if partIdx < 1 {
			continue
		}
		p := stdpath.Join(dir, o.GetName())
		l1, o1, e := Link(ctx, storage, p, args)
		if e != nil {
			err = errors.WithMessagef(e, "failed get [%s] link", p)
			break
		}
		ss1, e := stream.NewSeekableStream(&stream.FileStream{Ctx: ctx, Obj: o1}, l1)
		if e != nil {
			_ = l1.Close()
			err = errors.WithMessagef(e, "failed get [%s] stream", p)
			break
		}
		for partIdx >= len(ret) {
			ret = append(ret, nil)
		}
		ret[partIdx] = ss1
	}
	closeAll := func(r []*stream.SeekableStream) {
		for _, s := range r {
			if s != nil {
				_ = s.Close()
			}
		}
	}
	if err != nil {
		closeAll(ret)
		return nil, nil, nil, err
	}
	for i, ss1 := range ret {
		if ss1 == nil {
			closeAll(ret)
			return nil, nil, nil, errors.Errorf("failed merge [%s] parts, missing part %d", path, i)
		}
	}
	return obj, t, ret, nil
}

func ArchiveDecompress(ctx context.Context, storage driver.Driver, srcPath, dstDirPath string, args model.ArchiveDecompressArgs, lazyCache ...bool) error {
	if storage.Config().CheckStatus && storage.GetStorage().Status != WORK {
		return errors.WithMessagef(errs.StorageNotInit, "storage status: %s", storage.GetStorage().Status)
	}
	srcPath = utils.FixAndCleanPath(srcPath)
	dstDirPath = utils.FixAndCleanPath(dstDirPath)
	srcObj, err := GetUnwrap(ctx, storage, srcPath)
	if err != nil {
		return errors.WithMessage(err, "failed to get src object")
	}
	dstDir, err := GetUnwrap(ctx, storage, dstDirPath)
	if err != nil {
		return errors.WithMessage(err, "failed to get dst dir")
	}

	var newObjs []model.Obj
	switch s := storage.(type) {
	case driver.ArchiveDecompressResult:
		newObjs, err = s.ArchiveDecompress(ctx, srcObj, dstDir, args)
		if err == nil {
			if len(newObjs) > 0 {
				if !storage.Config().NoCache {
					if cache, exist := Cache.dirCache.Get(Key(storage, dstDirPath)); exist {
						for _, newObj := range newObjs {
							cache.UpdateObject(newObj.GetName(), newObj)
						}
					}
				}
			} else if !utils.IsBool(lazyCache...) {
				Cache.DeleteDirectory(storage, dstDirPath)
			}
		}
	case driver.ArchiveDecompress:
		err = s.ArchiveDecompress(ctx, srcObj, dstDir, args)
		if err == nil && !utils.IsBool(lazyCache...) {
			Cache.DeleteDirectory(storage, dstDirPath)
		}
	default:
		return errs.NotImplement
	}
	if !utils.IsBool(lazyCache...) && err == nil && needHandleObjsUpdateHook() {
		onlyList := false
		targetPath := dstDirPath
		if len(newObjs) == 1 && newObjs[0].IsDir() {
			targetPath = stdpath.Join(dstDirPath, newObjs[0].GetName())
		} else if len(newObjs) == 1 && !newObjs[0].IsDir() {
			onlyList = true
		} else if args.PutIntoNewDir {
			targetPath = stdpath.Join(dstDirPath, strings.TrimSuffix(srcObj.GetName(), stdpath.Ext(srcObj.GetName())))
		} else if innerBase := stdpath.Base(args.InnerPath); innerBase != "." && innerBase != "/" {
			targetPath = stdpath.Join(dstDirPath, innerBase)
			dstObj, e := Get(ctx, storage, targetPath)
			onlyList = e != nil || !dstObj.IsDir()
		}
		if onlyList {
			go List(context.Background(), storage, dstDirPath, model.ListArgs{Refresh: true})
		} else {
			var limiter *rate.Limiter
			if l, _ := GetSettingItemByKey(conf.HandleHookRateLimit); l != nil {
				if f, e := strconv.ParseFloat(l.Value, 64); e == nil && f > .0 {
					limiter = rate.NewLimiter(rate.Limit(f), 1)
				}
			}
			go RecursivelyListStorage(context.Background(), storage, targetPath, limiter, nil)
		}
	}
	return errors.WithStack(err)
}
