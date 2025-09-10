package main

import (
	"fmt"

	"roar/internal/app"
	"roar/internal/pkg/logger"

	"github.com/spf13/pflag"
)

var version = "dev"

func main() {
	logger.InitLogger()
	logger.Log.SetFormatter(&CustomFormatter{})

	versionFlag := pflag.BoolP("version", "v", false, "Print version information and exit")
	cfg := app.Config{}
	pflag.StringVarP(&cfg.ChartPath, "chart-path", "c", "", "Path to the app-of-apps Helm chart (required)")
	pflag.StringSliceVarP(&cfg.ValuesFiles, "values", "f", []string{}, "Path to a values file for the app-of-apps chart (can be repeated)")
	pflag.StringVarP(&cfg.OutputDir, "output-dir", "o", "rendered", "Directory to save rendered manifests")
	pflag.Parse()

	if *versionFlag {
		fmt.Printf("roar version: %s\n", version)
		return
	}

	if cfg.ChartPath == "" {
		logger.Log.Fatal("--chart-path is a required flag")
	}

	if err := app.Run(cfg); err != nil {
		logger.Log.Fatalf("Application failed: %v", err)
	}
}
