package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"werf-argo-renderer/internal/pkg/argo"
	"werf-argo-renderer/internal/pkg/git"
	"werf-argo-renderer/internal/pkg/helm"

	"github.com/sirupsen/logrus"
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
	logrus.Infof("Using temporary directory for clones: %s", tempDir)

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
			logrus.WithField("application", app.Metadata.Name).Errorf("Could not process application: %v. Skipping.", err)
		}
	}

	logrus.Info("All done!")
	return nil
}

func renderAndParseAppOfApps(chartPath string, valuesFiles []string) ([]argo.Application, error) {
	logrus.Info("Rendering the main 'app-of-apps' chart...")
	appOfAppsOpts := helm.RenderOptions{ReleaseName: "app-of-apps", ChartPath: chartPath, ValuesFiles: valuesFiles}
	appOfAppsManifests, err := helm.Template(appOfAppsOpts)

	if err != nil {
		return nil, fmt.Errorf("failed to render app-of-apps chart: %w", err)
	}

	logrus.Info("Parsing for Argo CD applications...")
	applications, err := argo.ParseApplications(appOfAppsManifests)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Argo applications: %w", err)
	}

	logrus.Infof("Found %d applications to process.", len(applications))
	return applications, nil
}

func processApplication(app argo.Application, state *appState) error {
	logCtx := logrus.WithField("application", app.Metadata.Name)
	logCtx.Info("Processing application...")

	var werfSetValues map[string]string
	var werfValuesFiles []string

	if app.Spec.Source.Plugin != nil {
		werfSetValues, werfValuesFiles = processPluginEnv(app.Spec.Source.Plugin.Env)
		logCtx.Infof("Found %d --set values and %d --values files in manifest.", len(werfSetValues), len(werfValuesFiles))
	}

	repoURL, ok := app.Metadata.Annotations["rawRepository"]
	if !ok || repoURL == "" {
		return fmt.Errorf("missing required annotation 'rawRepository' or it is empty")
	}

	chartSubPath, ok := app.Metadata.Annotations["rawPath"]
	if !ok {
		logCtx.Warn("Annotation 'rawPath' is missing. Defaulting to repository root ('.').")
		chartSubPath = "."
	}

	sshURL, err := convertHTTPtoSSH(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repo URL '%s': %w", repoURL, err)
	}

	cacheKey := fmt.Sprintf("%s@%s", sshURL, app.Spec.Source.TargetRevision)
	repoPath, isCached := state.clonedRepos[cacheKey]
	if !isCached {
		state.cloneCounter++
		repoPath = filepath.Join(state.tempDir, fmt.Sprintf("clone-%d", state.cloneCounter))
		logCtx.Infof("Cloning %s to %s", cacheKey, repoPath)
		err = git.Clone(sshURL, app.Spec.Source.TargetRevision, repoPath)

		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
		state.clonedRepos[cacheKey] = repoPath
	} else {
		logCtx.Infof("Using cached repository from path: %s", repoPath)
	}

	appServicePath := filepath.Join(repoPath, chartSubPath)
	appChartPath := filepath.Join(appServicePath, ".helm")
	absoluteValuesFiles := make([]string, len(werfValuesFiles))

	for i, file := range werfValuesFiles {
		absoluteValuesFiles[i] = filepath.Join(appServicePath, file)
	}

	appOpts := helm.RenderOptions{ReleaseName: app.Metadata.Name, ChartPath: appChartPath, ValuesFiles: absoluteValuesFiles, SetValues: werfSetValues}
	renderedApp, err := helm.Template(appOpts)
	if err != nil {
		return fmt.Errorf("failed to render chart: %w", err)
	}

	finalOutputDir := state.outputDir
	if env, ok := app.Metadata.Labels["env"]; ok && env != "" {
		finalOutputDir = filepath.Join(finalOutputDir, env)
		logCtx.Infof("Found 'env' label: '%s'. Saving to environment directory.", env)
	}

	if instance, ok := app.Metadata.Labels["instance"]; ok && instance != "" {
		finalOutputDir = filepath.Join(finalOutputDir, instance)
		logCtx.Infof("Found 'instance' label: '%s'. Saving to instance subdirectory.", instance)
	}

	if err := os.MkdirAll(finalOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output subdirectory %s: %w", finalOutputDir, err)
	}

	outputFile := filepath.Join(finalOutputDir, fmt.Sprintf("%s.yaml", app.Metadata.Name))
	err = os.WriteFile(outputFile, renderedApp, 0644)
	if err != nil {
		return fmt.Errorf("failed to write manifest to %s: %w", outputFile, err)
	}
	logCtx.Infof("Successfully rendered and saved manifest to %s", outputFile)
	return nil
}

type indexedFile struct {
	index int
	path  string
}

func processPluginEnv(envVars []argo.EnvVar) (map[string]string, []string) {
	setValues := make(map[string]string)
	var indexedValues []indexedFile
	for _, env := range envVars {
		if strings.HasPrefix(env.Name, "WERF_SET_") {
			parts := strings.SplitN(env.Value, "=", 2)
			if len(parts) == 2 {
				setValues[parts[0]] = parts[1]
			} else {
				logrus.Warnf("Skipping invalid WERF_SET variable '%s' with value '%s'", env.Name, env.Value)
			}
		} else if strings.HasPrefix(env.Name, "WERF_VALUES_") {
			indexStr := strings.TrimPrefix(env.Name, "WERF_VALUES_")
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				logrus.Warnf("Could not parse index from '%s'. Skipping.", env.Name)
				continue
			}
			indexedValues = append(indexedValues, indexedFile{index: index, path: env.Value})
		}
	}

	sort.Slice(indexedValues, func(i, j int) bool { return indexedValues[i].index < indexedValues[j].index })
	var valuesFiles []string

	for _, file := range indexedValues {
		valuesFiles = append(valuesFiles, file.path)
	}
	return setValues, valuesFiles
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
