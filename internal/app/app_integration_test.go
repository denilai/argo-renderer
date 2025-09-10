//go:build integration
// +build integration

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/stretchr/testify/require"
)

func setupIntegrationTest(t *testing.T) (string, func()) {
	binDir := t.TempDir()
	cmdLogPath := filepath.Join(t.TempDir(), "commands.log")

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
    echo "name: %s"
fi
`, cmdLogPath, "%s")
	require.NoError(t, os.WriteFile(helmScriptPath, []byte(helmScript), 0755))

	originalPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binDir, originalPath)
	os.Setenv("PATH", newPath)

	return cmdLogPath, func() {
		os.Setenv("PATH", originalPath)
	}
}

// createFakeGitRepo создает локальный Git репозиторий для тестов.
func createFakeGitRepo(t *testing.T) string {
	// 1. Создаем директорию, которая будет нашим "удаленным" репозиторием
	repoPath := t.TempDir()

	// 2. Инициализируем в ней Git репозиторий
	r, err := git.PlainInit(repoPath, false)
	require.NoError(t, err)

	// 3. Получаем рабочее дерево (worktree)
	w, err := r.Worktree()
	require.NoError(t, err)

	// 4. Создаем структуру файлов, которую ожидает наше приложение
	// Например, stable/my-service/.helm/Chart.yaml
	serviceDir := filepath.Join(repoPath, "stable", "my-service")
	helmDir := filepath.Join(serviceDir, ".helm")
	require.NoError(t, os.MkdirAll(helmDir, 0755))
	// Добавляем минимальный Chart.yaml, чтобы helm мог его обработать
	chartContent := "apiVersion: v2\nname: my-service-chart\nversion: 1.0.0"
	require.NoError(t, os.WriteFile(filepath.Join(helmDir, "Chart.yaml"), []byte(chartContent), 0644))

	// 5. Добавляем все созданные файлы в индекс Git
	_, err = w.Add(".")
	require.NoError(t, err)

	// 6. Создаем коммит. Это важно, так как go-git будет искать коммиты.
	_, err = w.Commit("Initial commit with service chart", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	return repoPath
}

func TestAppRun_Integration(t *testing.T) {
	cmdLogPath, cleanup := setupIntegrationTest(t)
	defer cleanup()

	// 1. Готовим входные данные
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	fakeRepoPath := createFakeGitRepo(t)
	appOfAppsDir := filepath.Join(tempDir, "app-of-apps-chart")
	require.NoError(t, os.MkdirAll(filepath.Join(appOfAppsDir, "templates"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(appOfAppsDir, "Chart.yaml"), []byte("apiVersion: v2\nname: fake-chart\nversion: 0.1.0"), 0644))
	// В шаблоне приложения мы теперь используем путь к нашему фейковому репозиторию
	// вместо реального URL. TargetRevision мы поменяем на "master", т.к. go-git
	// по умолчанию создает ветку master.
	appOfAppsTemplate := fmt.Sprintf(`
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dev-inf1-my-service
  labels:
    env: dev
    instance: inf1
  annotations:
    rawRepository: "%s" # <-- Используем локальный путь
    rawPath: "stable/my-service"
spec:
  source:
    targetRevision: master # <-- go-git по умолчанию создает master
    plugin:
      env:
        - name: WERF_SET_REPLICA_COUNT
          value: "global.replicaCount=3"
`, fakeRepoPath)
	require.NoError(t, os.WriteFile(filepath.Join(appOfAppsDir, "templates", "app.yaml"), []byte(appOfAppsTemplate), 0644))

	// 2. Запускаем основную логику приложения
	cfg := Config{ChartPath: appOfAppsDir, OutputDir: outputDir}
	err := Run(cfg)
	require.NoError(t, err)

	// 3. Проверяем результаты
	expectedOutputFile := filepath.Join(outputDir, "dev", "inf1", "dev-inf1-my-service.yaml")
	require.FileExists(t, expectedOutputFile)

	outputContent, err := os.ReadFile(expectedOutputFile)
	require.NoError(t, err)
	require.Contains(t, string(outputContent), "kind: FakedHelmOutputForApp")

	// Проверяем, что были вызваны правильные команды
	cmdLogContent, err := os.ReadFile(cmdLogPath)
	require.NoError(t, err)
	cmdLog := string(cmdLogContent)

	// Мы больше не можем проверить вызов 'git clone', так как его нет.
	// Но мы можем быть уверены, что клонирование прошло успешно, потому что
	// был вызван 'helm template' для дочернего приложения.
	require.NotContains(t, cmdLog, "git clone") // Убеждаемся, что git не вызывался
	require.Contains(t, cmdLog, "helm template dev-inf1-my-service")
	require.Contains(t, cmdLog, "--set global.replicaCount=3")
	// Проверяем, что helm был вызван с правильным путем, который был создан после "клонирования"
	require.Contains(t, cmdLog, filepath.Join(tempDir, "argo-charts-")) // Проверяем, что helm работает во временной директории
}
