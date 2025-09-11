// /roar/internal/pkg/helm/helm.go
package helm

import (
	"fmt"
	"os/exec"
	"roar/internal/pkg/logger"
	"strings"
)

type RenderOptions struct {
	ReleaseName string
	ChartPath   string
	ValuesFiles []string
	SetValues   map[string]string
}

func Template(opts RenderOptions) ([]byte, error) {
	args := []string{"template"}
	if opts.ReleaseName != "" {
		args = append(args, opts.ReleaseName)
	}
	args = append(args, opts.ChartPath)
	for _, valuesFile := range opts.ValuesFiles {
		args = append(args, "--values", valuesFile)
	}
	for key, value := range opts.SetValues {
		setValue := strings.Join([]string{key, value}, "=")
		args = append(args, "--set", setValue)
	}
	cmd := exec.Command("helm", args...)
	logger.Log.WithField("cmd", cmd.String()).Info("[CMD]")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("helm template failed: %w\nOutput:\n%s", err, string(output))
	}
	return output, nil
}
