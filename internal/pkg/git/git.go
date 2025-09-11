package git

import (
	"fmt"
	"roar/internal/pkg/logger"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func Clone(repoURL, revision, targetPath string) error {
	logCtx := logger.Log.WithField("repo", repoURL).WithField("revision", revision)
	logCtx.Info("Cloning repository using go-git...")

	opts := &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(revision),
		SingleBranch:  true,
		Depth:         1,
		Progress:      nil,
	}

	_, err := git.PlainClone(targetPath, false, opts)
	if err != nil {
		return fmt.Errorf("go-git clone failed for %s (revision %s): %w", repoURL, revision, err)
	}

	logCtx.Info("Successfully cloned repository.")
	return nil
}
