package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"roar/internal/pkg/argo"
	"roar/internal/pkg/git"
	"roar/internal/pkg/helm"
	"roar/internal/pkg/logger"
)

type Config struct {
	ChartPath   string
	ValuesFiles []string
	OutputDir   string
}

type appState struct {
	tempDir      string
	outputDir    string
	clonedRepos  map[string]string
	cloneCounter int
}

func Run(cfg Config) error {
	tempDir, err := os.MkdirTemp("", "argo-charts-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	logger.Log.Infof("Using temporary directory for clones: %s", tempDir)

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", cfg.OutputDir, err)
	}

	applications, err := renderAndParseAppOfApps(cfg.ChartPath, cfg.ValuesFiles)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	state := &appState{
		tempDir:     tempDir,
		outputDir:   cfg.OutputDir,
		clonedRepos: make(map[string]string),
	}

	for _, app := range applications {
		err := processApplication(app, state)
		if err != nil {
			logger.Log.WithField("application", app.Name).Errorf("Could not process application: %v. Skipping.", err)
		}
	}

	logger.Log.Info("All done!")
	return nil
}

func renderAndParseAppOfApps(chartPath string, valuesFiles []string) ([]argo.Application, error) {
	logger.Log.Info("Rendering the main 'app-of-apps' chart...")
	appOfAppsOpts := helm.RenderOptions{ReleaseName: "app-of-apps", ChartPath: chartPath, ValuesFiles: valuesFiles}
	appOfAppsManifests, err := helm.Template(appOfAppsOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to render app-of-apps chart: %w", err)
	}

	logger.Log.Info("Parsing for Argo CD applications...")
	applications, err := argo.ParseApplications(appOfAppsManifests)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Argo applications: %w", err)
	}

	logger.Log.Infof("Found %d applications to process.", len(applications))
	return applications, nil
}

func processApplication(app argo.Application, state *appState) error {
	logCtx := logger.Log.WithField("application", app.Name)
	logCtx.Info("Processing application...")

	werfSetValues := app.Setters
	if werfSetValues == nil {
		werfSetValues = make(map[string]string)
	}

	logCtx.Infof("Found %d --set values and %d --values files.", len(werfSetValues), len(app.ValuesFiles))

	if app.Instance != "" {
		werfSetValues["global.instance"] = app.Instance
		logCtx.Infof("Resolved final 'instance' to '%s'", app.Instance)
	}
	if app.Env != "" {
		werfSetValues["global.env"] = app.Env
		logCtx.Infof("Resolved final 'env' to '%s'", app.Env)
	}

	sshURL, err := convertHTTPtoSSH(app.RepoURL)
	if err != nil {
		return fmt.Errorf("invalid repo URL '%s': %w", app.RepoURL, err)
	}

	cacheKey := fmt.Sprintf("%s@%s", sshURL, app.TargetRevision)
	repoPath, isCached := state.clonedRepos[cacheKey]
	if !isCached {
		state.cloneCounter++
		repoPath = filepath.Join(state.tempDir, fmt.Sprintf("clone-%d", state.cloneCounter))
		logCtx.Infof("Cloning %s to %s", cacheKey, repoPath)
		err = git.Clone(sshURL, app.TargetRevision, repoPath)
		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
		state.clonedRepos[cacheKey] = repoPath
	} else {
		logCtx.Infof("Using cached repository from path: %s", repoPath)
	}

	appServicePath := filepath.Join(repoPath, app.Path)
	appChartPath := filepath.Join(appServicePath, ".helm")
	absoluteValuesFiles := make([]string, len(app.ValuesFiles))
	for i, file := range app.ValuesFiles {
		absoluteValuesFiles[i] = filepath.Join(appServicePath, file)
	}

	appOpts := helm.RenderOptions{ReleaseName: app.Name, ChartPath: appChartPath, ValuesFiles: absoluteValuesFiles, SetValues: werfSetValues}
	renderedApp, err := helm.Template(appOpts)
	if err != nil {
		return fmt.Errorf("failed to render chart: %w", err)
	}

	finalOutputDir := state.outputDir
	if app.Env != "" {
		finalOutputDir = filepath.Join(finalOutputDir, app.Env)
		logCtx.Infof("Using resolved 'env': '%s' for output directory.", app.Env)
	}
	if app.Instance != "" {
		finalOutputDir = filepath.Join(finalOutputDir, app.Instance)
		logCtx.Infof("Using resolved 'instance': '%s' for output directory.", app.Instance)
	}

	if err := os.MkdirAll(finalOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output subdirectory %s: %w", finalOutputDir, err)
	}

	outputFile := filepath.Join(finalOutputDir, fmt.Sprintf("%s.yaml", app.Name))
	err = os.WriteFile(outputFile, renderedApp, 0644)
	if err != nil {
		return fmt.Errorf("failed to write manifest to %s: %w", outputFile, err)
	}
	logCtx.Infof("Successfully rendered and saved manifest to %s", outputFile)
	return nil
}

func convertHTTPtoSSH(httpURL string) (string, error) {
	if strings.HasPrefix(httpURL, "git@") {
		return httpURL, nil
	}
	parsedURL, err := url.Parse(httpURL)
	if err != nil {
		return "", fmt.Errorf("could not parse URL: %w", err)
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return httpURL, nil
	}
	path := strings.TrimPrefix(parsedURL.Path, "/")
	sshURL := fmt.Sprintf("git@%s:%s", parsedURL.Host, path)
	return sshURL, nil
}
