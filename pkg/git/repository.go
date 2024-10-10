package git

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

type Repository struct {
	URL url.URL
}

func NewRepository(u url.URL) *Repository {
	return &Repository{URL: u}
}

func NewRepositoryFromPath(path string) (*Repository, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = "file"
	}
	if u.Scheme != "file" {
		return nil, errors.New("path must be a file url")
	}
	return &Repository{URL: *u}, nil
}

func copyLocalRepo(fs afero.Fs, src, dst url.URL) error {
	// TODO: why is this so hard?
	if src.Scheme != "file" {
		return errors.New("src scheme must be 'file'")
	}
	if dst.Scheme != "file" {
		return errors.New("dst scheme must be 'file'")
	}

	return afero.Walk(fs, src.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src.Path, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst.Path, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}
