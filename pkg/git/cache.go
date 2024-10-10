package git

import (
	"net/url"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

// TODO: think about concurrency
type Cache interface {
	Get(url url.URL) (*Repository, error)
}

type FsCache struct {
	fs       afero.Fs
	basePath string
	load     func(src url.URL, dst string) (*Repository, error)
}

func NewFsCache(fs afero.Fs, basePath string, load func(src url.URL, dst string) (*Repository, error)) *FsCache {
	return &FsCache{fs: fs, basePath: basePath, load: load}
}

func (s *FsCache) Get(url url.URL) (*Repository, error) {
	log.Debug().Str("url", url.String()).Msg("checking git repository cache")
	path := uriToPath(url)
	fullPath := filepath.Join(s.basePath, path)
	exists, err := afero.Exists(s.fs, fullPath)
	if err != nil {
		return nil, err
	}
	log.Debug().Str("url", url.String()).Bool("hit", exists).Msg("cache status")
	if !exists {
		log.Debug().Str("url", url.String()).Msg("cloning repository from remote")
		return s.load(url, fullPath)
	}
	return NewRepositoryFromPath(fullPath)
}

func uriToPath(url url.URL) string {
	return filepath.Join(url.Hostname(), url.Path)
}
