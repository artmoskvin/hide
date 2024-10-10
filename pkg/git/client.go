package git

import (
	"fmt"
	"net/url"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Client interface {
	Checkout(repo *Repository, ref string) error
	Clone(repo *Repository, dst url.URL) (*Repository, error)
	IsUpToDate(repo *Repository) (bool, error)
	Pull(repo *Repository) error
}

type ClientImpl struct {
	// TODO: use billy?
}

func NewClientImpl() Client {
	return &ClientImpl{}
}

func (c *ClientImpl) Checkout(repo *Repository, ref string) error {
	if repo.URL.Scheme != "file" {
		return fmt.Errorf("repo url scheme must be 'file'")
	}
	repoPath := repo.URL.Path
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(ref),
	})
	return err
}

func (c *ClientImpl) Clone(repo *Repository, dst url.URL) (*Repository, error) {
	if dst.Scheme != "file" {
		return nil, fmt.Errorf("dst scheme must be 'file'")
	}
	_, err := git.PlainClone(dst.Path, false, &git.CloneOptions{
		URL:      repo.URL.String(),
		Progress: os.Stdout,         // TODO: remove or use a logger
	})
	if err != nil {
		return nil, err
	}
	return NewRepository(dst), nil
}

func (c *ClientImpl) IsUpToDate(repo *Repository) (bool, error) {
	if repo.URL.Scheme != "file" {
		return false, fmt.Errorf("repo scheme must be 'file'")
	}

	// Open repo
	r, err := git.PlainOpen(repo.URL.Path)
	if err != nil {
		return false, err
	}

	// Get current HEAD
	head, err := r.Head()
	if err != nil {
		return false, err
	}

	// Get remote
	remote, err := r.Remote("origin")
	if err != nil {
		return false, err
	}

	// List remote references
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return false, err
	}

	// Find the matching remote branch
	var remoteRef *plumbing.Reference
	for _, ref := range refs {
		if ref.Name() == plumbing.ReferenceName("refs/heads/"+head.Name().Short()) {
			remoteRef = ref
			break
		}
	}

	if remoteRef == nil {
		return false, fmt.Errorf("remote branch not found")
	}

	if head.Hash() == remoteRef.Hash() {
		return true, nil
	}

	return false, nil
}

// Pulls local repository
func (c *ClientImpl) Pull(repo *Repository) error {
	// Open the repository
	if repo.URL.Scheme != "file" {
		return fmt.Errorf("repo scheme must be 'file'")
	}
	r, err := git.PlainOpen(repo.URL.Path)
	if err != nil {
		return err
	}

	// Get the worktree
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Pull the latest changes
	err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func Clone(src url.URL, dst string) (*Repository, error) {
	// if dst.Scheme != "file" {
	// 	return nil, fmt.Errorf("dst scheme must be 'file'")
	// }

	_, err := git.PlainClone(dst, false, &git.CloneOptions{
		URL:      src.String(),
		Progress: os.Stdout,    // TODO: remove or use a logger
	})
	if err != nil {
		return nil, err
	}
	return NewRepositoryFromPath(dst)
}
