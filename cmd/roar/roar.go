package main

import (
	"fmt"
	"os"

	"roar/internal/app"
	"roar/internal/pkg/logger"

	"github.com/spf13/pflag"
)

var version = "dev"

func main() {

	versionFlag := pflag.BoolP("version", "v", false, "Print version information and exit")
	cfg := app.Config{}
	// pflag.StringVar(&cfg.ChartPath, "chart-path", "", "Path to the app-of-apps Helm chart (required)")
	pflag.StringSliceVarP(&cfg.ValuesFiles, "values", "f", []string{}, "Path to a values file for the app-of-apps chart (can be repeated)")
	pflag.StringVarP(&cfg.OutputDir, "output-dir", "o", "rendered", "Directory to save rendered manifests")
	pflag.StringVarP(&cfg.LogLevel, "log-level", "l", "warn", "Log level (debug, info, warn, error)")

	roar := "roar"

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [CHART_PATH] [flags]\n\n", roar)
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  CHART_PATH   Path to the app-of-apps Helm chart (required)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	logger.InitLogger()
	logger.Log.SetLevel(logger.ParseLogLevel(cfg.LogLevel))
	logger.Log.SetFormatter(&CustomFormatter{})

	if *versionFlag {
		fmt.Printf("roar version: %s\n", version)
		return
	}

	args := pflag.Args()

	if len(args) != 1 {
		logger.Log.Error("Error: exactly one argument [CHART_PATH] is required.")
		pflag.Usage()
		os.Exit(1)
	}

	cfg.ChartPath = args[0]

	if err := app.Run(cfg); err != nil {
		logger.Log.Fatalf("Application failed: %v", err)
	}
}
