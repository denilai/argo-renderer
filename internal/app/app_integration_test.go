//go:build integration
// +build integration

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupIntegrationTest создает временное окружение с УМНЫМИ фейковыми git и helm
func setupIntegrationTest(t *testing.T) (string, func()) {
	binDir := t.TempDir()
	cmdLogPath := filepath.Join(t.TempDir(), "commands.log")

	// Создаем фейковый git
	gitScriptPath := filepath.Join(binDir, "git")
	gitScript := fmt.Sprintf(`#!/bin/bash
echo "git $@" >> %s
if [ "$1" == "clone" ]; then TARGET_DIR="${@: -1}"; mkdir -p "$TARGET_DIR"; fi`, cmdLogPath)
	require.NoError(t, os.WriteFile(gitScriptPath, []byte(gitScript), 0755))

	helmScriptPath := filepath.Join(binDir, "helm")
	helmScript := fmt.Sprintf(`#!/bin/bash
echo "helm $@" >> %s

RELEASE_NAME=$2
CHART_PATH=$3

if [ "$RELEASE_NAME" == "app-of-apps" ]; then
    if [ -d "${CHART_PATH}/templates" ]; then
        cat "${CHART_PATH}"/templates/*.yaml
    fi
else
    echo "kind: FakedHelmOutputForApp"
fi
`, cmdLogPath)
	require.NoError(t, os.WriteFile(helmScriptPath, []byte(helmScript), 0755))

	originalPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binDir, originalPath)
	os.Setenv("PATH", newPath)

	return cmdLogPath, func() {
		os.Setenv("PATH", originalPath)
	}
}

func TestAppRun_Integration(t *testing.T) {
	cmdLogPath, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// 1. Готовим входные данные
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	appOfAppsDir := filepath.Join(tempDir, "app-of-apps-chart")
	require.NoError(t, os.MkdirAll(filepath.Join(appOfAppsDir, "templates"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(appOfAppsDir, "Chart.yaml"), []byte("apiVersion: v2\nname: fake-chart\nversion: 0.1.0"), 0644))
	appOfAppsTemplate := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dev-inf1-my-service
  labels: {env: dev, instance: inf1}
  annotations:
    rawRepository: "https://gitlab.com/my-org/my-product.git"
    rawPath: "stable/my-service"
spec:
  source:
    targetRevision: feature-branch
    plugin:
      env:
        - name: WERF_SET_REPLICA_COUNT
          value: "global.replicaCount=3"
`
	require.NoError(t, os.WriteFile(filepath.Join(appOfAppsDir, "templates", "app.yaml"), []byte(appOfAppsTemplate), 0644))

	// 2. Запускаем основную логику приложения
	cfg := Config{ChartPath: appOfAppsDir, OutputDir: outputDir}
	fmt.Print(cfg)
	err := Run(cfg)
	require.NoError(t, err)

	// 3. Проверяем результаты
	expectedOutputFile := filepath.Join(outputDir, "dev", "inf1", "dev-inf1-my-service.yaml")
	require.FileExists(t, expectedOutputFile)

	// Проверяем, что внутри файла правильный (фейковый) контент для дочернего приложения
	outputContent, err := os.ReadFile(expectedOutputFile)
	require.NoError(t, err)
	require.Contains(t, string(outputContent), "kind: FakedHelmOutputForApp")

	// Проверяем, что были вызваны правильные команды
	cmdLogContent, err := os.ReadFile(cmdLogPath)
	require.NoError(t, err)
	cmdLog := string(cmdLogContent)

	require.Contains(t, cmdLog, "git clone --branch feature-branch")
	require.Contains(t, cmdLog, "helm template dev-inf1-my-service")
	require.Contains(t, cmdLog, "--set global.replicaCount=3")
}
