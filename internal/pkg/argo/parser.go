package argo

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"roar/internal/pkg/logger"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Application struct {
	Name           string
	Instance       string
	Env            string
	RepoURL        string
	Path           string
	TargetRevision string
	PluginEnv      []EnvVar
	ValuesFiles    []string
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type rawApplication struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string            `yaml:"name"`
		Labels      map[string]string `yaml:"labels"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Source struct {
			RepoURL        string `yaml:"repoURL"`
			TargetRevision string `yaml:"targetRevision"`
			Path           string `yaml:"path"`
			Plugin         *struct {
				Env []EnvVar `yaml:"env"`
			} `yaml:"plugin"`
		} `yaml:"source"`
	} `yaml:"spec"`
}

func ParseApplications(yamlData []byte) ([]Application, error) {
	var finalApps []Application
	decoder := yaml.NewDecoder(bytes.NewReader(yamlData))

	for {
		var rawApp rawApplication
		err := decoder.Decode(&rawApp)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode yaml document: %w", err)
		}

		if rawApp.ApiVersion == "argoproj.io/v1alpha1" && rawApp.Kind == "Application" {
			logCtx := logger.Log.WithField("application", rawApp.Metadata.Name)
			cleanApp, err := newApplicationFromRaw(rawApp, logCtx)
			if err != nil {
				return nil, fmt.Errorf("application '%s' is invalid: %w", rawApp.Metadata.Name, err)
			}
			finalApps = append(finalApps, cleanApp)
		}
	}

	return finalApps, nil
}

func newApplicationFromRaw(raw rawApplication, logCtx *logrus.Entry) (Application, error) {
	app := Application{
		Name:           raw.Metadata.Name,
		TargetRevision: raw.Spec.Source.TargetRevision,
	}

	var instanceFromLabel, envFromLabel string
	if raw.Metadata.Labels != nil {
		if val, ok := raw.Metadata.Labels["instance"]; ok {
			instanceFromLabel = val
		}
		if val, ok := raw.Metadata.Labels["env"]; ok {
			envFromLabel = val
		}
	}

	var instanceFromPlugin, envFromPlugin string
	var remainingPluginEnv []EnvVar

	type indexedValueFile struct {
		index int
		path  string
	}
	var indexedValues []indexedValueFile

	if raw.Spec.Source.Plugin != nil {
		for _, envVar := range raw.Spec.Source.Plugin.Env {
			switch {
			case envVar.Name == "WERF_SET_INSTANCE":
				instanceFromPlugin = extractValueFromWerfSet(envVar.Value)
				remainingPluginEnv = append(remainingPluginEnv, envVar)
			case envVar.Name == "WERF_SET_ENV":
				envFromPlugin = extractValueFromWerfSet(envVar.Value)
				remainingPluginEnv = append(remainingPluginEnv, envVar)
			case strings.HasPrefix(envVar.Name, "WERF_VALUES_"):
				indexStr := strings.TrimPrefix(envVar.Name, "WERF_VALUES_")
				index, err := strconv.Atoi(indexStr)
				if err != nil {
					logCtx.Warnf("Could not parse index from '%s'. Skipping.", envVar.Name)
					continue
				}
				indexedValues = append(indexedValues, indexedValueFile{index: index, path: envVar.Value})
			default:
				remainingPluginEnv = append(remainingPluginEnv, envVar)
			}
		}
	}
	app.PluginEnv = remainingPluginEnv

	sort.Slice(indexedValues, func(i, j int) bool {
		return indexedValues[i].index < indexedValues[j].index
	})
	app.ValuesFiles = make([]string, len(indexedValues))
	for i, iv := range indexedValues {
		app.ValuesFiles[i] = iv.path
	}

	if instanceFromLabel != "" && instanceFromPlugin != "" && instanceFromLabel != instanceFromPlugin {
		return Application{}, fmt.Errorf("conflicting values for 'instance': label is '%s', plugin.env is '%s'", instanceFromLabel, instanceFromPlugin)
	}
	if instanceFromLabel != "" {
		app.Instance = instanceFromLabel
	} else {
		app.Instance = instanceFromPlugin
	}

	if envFromLabel != "" && envFromPlugin != "" && envFromLabel != envFromPlugin {
		return Application{}, fmt.Errorf("conflicting values for 'env': label is '%s', plugin.env is '%s'", envFromLabel, envFromPlugin)
	}
	if envFromLabel != "" {
		app.Env = envFromLabel
	} else {
		app.Env = envFromPlugin
	}

	repoURL, ok := raw.Metadata.Annotations["rawRepository"]
	if !ok || repoURL == "" {
		logCtx.Warnf("missing 'rawRepository' annotation. Falling back to spec.source.repoURL='%s'", raw.Spec.Source.RepoURL)
		repoURL = raw.Spec.Source.RepoURL
		if repoURL == "" {
			return Application{}, fmt.Errorf("both 'rawRepository' annotation and 'spec.source.repoURL' are empty")
		}
	}
	app.RepoURL = repoURL

	path, ok := raw.Metadata.Annotations["rawPath"]
	if !ok {
		logCtx.Warnf("missing 'rawPath' annotation. Falling back to spec.source.path='%s'", raw.Spec.Source.Path)
		path = raw.Spec.Source.Path
		if path == "" {
			logCtx.Warn("both 'rawPath' annotation and 'spec.source.path' are empty. Falling back to '.'")
			path = "."
		}
	}
	app.Path = path

	return app, nil
}

func extractValueFromWerfSet(s string) string {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
