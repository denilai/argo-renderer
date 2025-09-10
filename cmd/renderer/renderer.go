// /werf-argo-renderer/cmd/renderer/renderer.go

package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	// Импортируем наш новый пакет с бизнес-логикой
	"werf-argo-renderer/internal/app"
)

func main() {
	// 1. Настройка логгера
	logrus.SetFormatter(&CustomFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	// 2. Определение и парсинг флагов
	cfg := app.Config{}
	pflag.StringVarP(&cfg.ChartPath, "chart-path", "c", "", "Path to the app-of-apps Helm chart (required)")
	pflag.StringSliceVarP(&cfg.ValuesFiles, "values", "f", []string{}, "Path to a values file for the app-of-apps chart (can be repeated)")
	pflag.StringVarP(&cfg.OutputDir, "output-dir", "o", "rendered", "Directory to save rendered manifests")
	pflag.Parse()

	// 3. Валидация
	if cfg.ChartPath == "" {
		logrus.Fatal("--chart-path is a required flag")
	}

	// 4. Запуск основной логики приложения
	if err := app.Run(cfg); err != nil {
		logrus.Fatalf("Application failed: %v", err)
	}
}
