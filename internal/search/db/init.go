package db

import (
	"github.com/OpenListTeam/OpenList/v4/internal/search/searcher"
)

var config = searcher.Config{
	Name:       "database",
	AutoUpdate: true,
}

func init() {
	searcher.RegisterSearcher(config, func() (searcher.Searcher, error) {
		return &DB{}, nil
	})
}
