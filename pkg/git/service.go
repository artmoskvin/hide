package git

import (
	"net/url"

	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

type Service interface {
	Clone(url url.URL, dst url.URL) (*Repository, error)
	Checkout(repo *Repository, ref string) error
}

type ServiceImpl struct {
	client Client
	cache  Cache
}

func NewService(client Client, cache Cache) Service {
	return &ServiceImpl{client: client, cache: cache}
}

func (s *ServiceImpl) Clone(url url.URL, dst url.URL) (*Repository, error) {
	log.Debug().Str("url", url.String()).Str("dst", dst.String()).Msg("cloning git repository")
	repo, err := s.cache.Get(url)
	if err != nil {
		return nil, err
	}

	log.Debug().Str("url", url.String()).Msg("checking repository status")
	isUpToDate, err := s.client.IsUpToDate(repo)
	if err != nil {
		return nil, err
	}

	log.Debug().Str("url", url.String()).Bool("isUpToDate", isUpToDate).Msg("repository status")
	if !isUpToDate {
		log.Debug().Str("url", url.String()).Msg("pulling repository")
		err = s.client.Pull(repo)
		if err != nil {
			return nil, err
		}
		log.Debug().Str("url", url.String()).Msg("pulled repository")
	}

	// cloning a local repo is slower than copying it
	// r, err := s.client.Clone(repo, dst)
	// return r, err
	if err := copyLocalRepo(afero.NewOsFs(), repo.URL, dst); err == nil {
		log.Debug().Str("url", url.String()).Str("dst", dst.String()).Msg("cloned git repository")
	}
	return NewRepository(dst), err
}

func (s *ServiceImpl) Checkout(repo *Repository, ref string) error {
	log.Debug().Str("repo", repo.URL.String()).Str("ref", ref).Msg("checking out git repository")
	err := s.client.Checkout(repo, ref)
	log.Debug().Str("repo", repo.URL.String()).Str("ref", ref).Msg("checked out git repository")
	return err
}
