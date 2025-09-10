package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"roar/internal/pkg/argo"
	"roar/internal/pkg/git"
	"roar/internal/pkg/helm"
	"roar/internal/pkg/logger"
)

type Config struct {
	ChartPath   string
	ValuesFiles []string
	OutputDir   string
	LogLevel    string
}

type appState struct {
	mu           sync.Mutex
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

	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	errChan := make(chan error, len(applications))

	for _, app := range applications {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(app argo.Application) {
			defer wg.Done()
			// "Освобождаем" место в семафоре после завершения работы.
			defer func() { <-semaphore }()

			if err := processApplication(app, state); err != nil {
				errChan <- err
			}
		}(app)
	}

	wg.Wait()
	close(errChan)

	var allErrors []error
	for err := range errChan {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		logger.Log.Errorf("Completed with %d errors.", len(allErrors))
		return fmt.Errorf("failed to process %d application(s), first error: %w", len(allErrors), allErrors[0])
	}

	logger.Log.Info("Processing applications sequentially...")
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

	// --- НАЧАЛО КРИТИЧЕСКОЙ СЕКЦИИ ---
	state.mu.Lock()
	repoPath, isCached := state.clonedRepos[cacheKey]
	state.mu.Unlock()
	// --- КОНЕЦ КРИТИЧЕСКОЙ СЕКЦИИ ---

	if !isCached {
		// --- НАЧАЛО КРИТИЧЕСКОЙ СЕКЦИИ (для обновления счетчика и карты) ---
		state.mu.Lock()
		state.cloneCounter++
		repoPath = filepath.Join(state.tempDir, fmt.Sprintf("clone-%d", state.cloneCounter))
		state.mu.Unlock()
		// --- КОНЕЦ КРИТИЧЕСКОЙ СЕКЦИИ ---

		logCtx.Infof("Cloning %s to %s", cacheKey, repoPath)
		err = git.Clone(sshURL, app.TargetRevision, repoPath)
		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}

		// --- НАЧАЛО КРИТИЧЕСКОЙ СЕКЦИИ (для записи в кэш) ---
		state.mu.Lock()
		state.clonedRepos[cacheKey] = repoPath
		state.mu.Unlock()
		// --- КОНЕЦ КРИТИЧЕСКОЙ СЕКЦИИ ---
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
