// /werf-argo-renderer/internal/git/git.go

package git

import (
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"
)

func Clone(repoURL, revision, targetPath string) error {
	cmd := exec.Command(
		"git", "clone",
		"--branch", revision,
		"--single-branch",
		"--depth=1",
		repoURL,
		targetPath,
	)

	// ИЗМЕНЕНИЕ: Используем logrus с полями для структурированного лога
	logrus.WithField("cmd", cmd.String()).Info("Executing command")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed for %s (revision %s): %w\nOutput:\n%s", repoURL, revision, err, string(output))
	}
	return nil
}
