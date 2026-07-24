package dropbox

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/go-resty/resty/v2"
)

type Dropbox struct {
	model.Storage
	Addition
	base        string
	contentBase string
}

func (d *Dropbox) Config() driver.Config {
	return config
}

func (d *Dropbox) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Dropbox) Init(ctx context.Context) error {
	query := "foo"
	res, err := d.request("/2/check/user", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"query": query,
		})
	})
	if err != nil {
		return err
	}
	result := utils.Json.Get(res, "result").ToString()
	if result != query {
		return fmt.Errorf("failed to check user: %s", string(res))
	}
	d.RootNamespaceId, err = d.GetRootNamespaceId(ctx)

	return err
}

func (d *Dropbox) GetRootNamespaceId(ctx context.Context) (string, error) {
	res, err := d.request("/2/users/get_current_account", http.MethodPost, func(req *resty.Request) {
		req.SetBody(nil)
	})
	if err != nil {
		return "", err
	}
	var currentAccountResp CurrentAccountResp
	err = utils.Json.Unmarshal(res, &currentAccountResp)
	if err != nil {
		return "", err
	}
	rootNamespaceId := currentAccountResp.RootInfo.RootNamespaceId
	return rootNamespaceId, nil
}

func (d *Dropbox) Drop(ctx context.Context) error {
	return nil
}

func (d *Dropbox) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	files, err := d.getFiles(ctx, dir.GetPath())
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *Dropbox) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	res, err := d.request("/2/files/get_temporary_link", http.MethodPost, func(req *resty.Request) {
		req.SetContext(ctx).SetBody(base.Json{
			"path": file.GetPath(),
		})
	})
	if err != nil {
		return nil, err
	}
	url := utils.Json.Get(res, "link").ToString()
	exp := time.Hour
	return &model.Link{
		URL:        url,
		Expiration: &exp,
	}, nil
}

func (d *Dropbox) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	_, err := d.request("/2/files/create_folder_v2", http.MethodPost, func(req *resty.Request) {
		req.SetContext(ctx).SetBody(base.Json{
			"autorename": false,
			"path":       parentDir.GetPath() + "/" + dirName,
		})
	})
	return err
}

func (d *Dropbox) Move(ctx context.Context, srcObj, dstDir model.Obj) error {
	toPath := dstDir.GetPath() + "/" + srcObj.GetName()

	_, err := d.request("/2/files/move_v2", http.MethodPost, func(req *resty.Request) {
		req.SetContext(ctx).SetBody(base.Json{
			"allow_ownership_transfer": false,
			"allow_shared_folder":      false,
			"autorename":               false,
			"from_path":                srcObj.GetID(),
			"to_path":                  toPath,
		})
	})
	return err
}

func (d *Dropbox) Rename(ctx context.Context, srcObj model.Obj, newName string) error {
	path := srcObj.GetPath()
	fileName := srcObj.GetName()
	toPath := path[:len(path)-len(fileName)] + newName

	_, err := d.request("/2/files/move_v2", http.MethodPost, func(req *resty.Request) {
		req.SetContext(ctx).SetBody(base.Json{
			"allow_ownership_transfer": false,
			"allow_shared_folder":      false,
			"autorename":               false,
			"from_path":                srcObj.GetID(),
			"to_path":                  toPath,
		})
	})
	return err
}

func (d *Dropbox) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	toPath := dstDir.GetPath() + "/" + srcObj.GetName()
	_, err := d.request("/2/files/copy_v2", http.MethodPost, func(req *resty.Request) {
		req.SetContext(ctx).SetBody(base.Json{
			"allow_ownership_transfer": false,
			"allow_shared_folder":      false,
			"autorename":               false,
			"from_path":                srcObj.GetID(),
			"to_path":                  toPath,
		})
	})
	return err
}

func (d *Dropbox) Remove(ctx context.Context, obj model.Obj) error {
	uri := "/2/files/delete_v2"
	_, err := d.request(uri, http.MethodPost, func(req *resty.Request) {
		req.SetContext(ctx).SetBody(base.Json{
			"path": obj.GetID(),
		})
	})
	return err
}

func (d *Dropbox) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	if stream.GetSize() < 0 {
		if _, err := stream.CacheFullAndWriter(nil, nil); err != nil {
			return err
		}
	}

	toPath := dstDir.GetPath() + "/" + stream.GetName()
	const simpleUploadLimit = 150 * 1024 * 1024
	if stream.GetSize() <= simpleUploadLimit {
		return d.upload(ctx, toPath, stream, up)
	}

	const partSize = 64 * 1024 * 1024
	return d.uploadSession(ctx, toPath, stream, up, partSize)
}

var _ driver.Driver = (*Dropbox)(nil)
