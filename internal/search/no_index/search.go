package no_index

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/fs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/search/searcher"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
)

// maxResults caps how many matches a single search walks and returns. Once
// reached, the walk stops so a search over a huge tree can't run forever.
const maxResults = 1000

type NoIndex struct{}

func (NoIndex) Config() searcher.Config {
	return config
}

func (n NoIndex) Search(ctx context.Context, req model.SearchReq) ([]model.SearchNode, int64, error) {
	return n.SearchFiltered(ctx, req, nil)
}

type queueEntry struct {
	reqPath string
	obj     model.Obj
	depth   int
}

// SearchFiltered walks the tree under req.Parent with fs.List, so it goes
// through the same user base_path, Meta, hidden-file, and driver semantics
// as a normal listing. It applies filter (permission checks) before
// counting or paginating, so total and pages never leak entries the caller
// can't see.
func (NoIndex) SearchFiltered(ctx context.Context, req model.SearchReq, filter searcher.Filter) ([]model.SearchNode, int64, error) {
	keywords := strings.Fields(strings.ToLower(req.Keywords))
	ignorePaths := conf.SlicesMap[conf.IgnorePaths]
	maxDepth := setting.GetInt(conf.MaxIndexDepth, 20)

	root, err := fs.Get(ctx, req.Parent, &fs.GetArgs{})
	if err != nil {
		return nil, 0, err
	}

	var matched []model.SearchNode
	queue := []queueEntry{{reqPath: req.Parent, obj: root, depth: 0}}
	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}

		entry := queue[0]
		queue = queue[1:]

		if isIgnored(entry.reqPath, ignorePaths) {
			continue
		}

		if entry.reqPath != req.Parent && matchesKeywords(entry.obj.GetName(), keywords) &&
			matchesScope(entry.obj.IsDir(), req.Scope) {
			node := model.SearchNode{
				Parent: path.Dir(entry.reqPath),
				Name:   entry.obj.GetName(),
				IsDir:  entry.obj.IsDir(),
				Size:   entry.obj.GetSize(),
			}
			if filter == nil || filter(node) {
				matched = append(matched, node)
				if len(matched) >= maxResults {
					break
				}
			}
		}

		if !entry.obj.IsDir() || entry.depth >= maxDepth {
			continue
		}
		meta, _ := op.GetNearestMeta(entry.reqPath)
		children, err := fs.List(context.WithValue(ctx, conf.MetaKey, meta), entry.reqPath, &fs.ListArgs{})
		if err != nil {
			return nil, 0, err
		}
		for _, child := range children {
			queue = append(queue, queueEntry{
				reqPath: path.Join(entry.reqPath, child.GetName()),
				obj:     child,
				depth:   entry.depth + 1,
			})
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Name != matched[j].Name {
			return matched[i].Name < matched[j].Name
		}
		return path.Join(matched[i].Parent, matched[i].Name) < path.Join(matched[j].Parent, matched[j].Name)
	})

	total := int64(len(matched))
	from := (req.Page - 1) * req.PerPage
	if from >= len(matched) {
		return nil, total, nil
	}
	to := min(from+req.PerPage, len(matched))
	return matched[from:to], total, nil
}

func matchesKeywords(name string, keywords []string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range keywords {
		if !strings.Contains(lower, keyword) {
			return false
		}
	}
	return true
}

func matchesScope(isDir bool, scope int) bool {
	switch scope {
	case 1:
		return isDir
	case 2:
		return !isDir
	default:
		return true
	}
}

// isIgnored reports whether reqPath is one of ignorePaths or lies under one
// of them. It compares on path segment boundaries so that an ignore entry
// like /data/a does not also match /data/abc.
func isIgnored(reqPath string, ignorePaths []string) bool {
	for _, ignorePath := range ignorePaths {
		if ignorePath == "" {
			continue
		}
		if utils.IsSubPath(ignorePath, reqPath) {
			return true
		}
	}
	return false
}

func (NoIndex) Index(ctx context.Context, node model.SearchNode) error         { return nil }
func (NoIndex) BatchIndex(ctx context.Context, nodes []model.SearchNode) error { return nil }
func (NoIndex) Get(ctx context.Context, parent string) ([]model.SearchNode, error) {
	return nil, nil
}
func (NoIndex) Del(ctx context.Context, prefix string) error { return nil }
func (NoIndex) Release(ctx context.Context) error            { return nil }
func (NoIndex) Clear(ctx context.Context) error              { return nil }

var _ searcher.Searcher = (*NoIndex)(nil)
var _ searcher.FilteredSearcher = (*NoIndex)(nil)
