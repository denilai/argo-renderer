package argo

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Application struct {
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

			Path   string `yaml:"path"`
			Plugin *struct {
				Env []EnvVar `yaml:"env"`
			} `yaml:"plugin"`
		} `yaml:"source"`
	} `yaml:"spec"`
}

func ParseApplications(yamlData []byte) ([]Application, error) {
	var apps []Application
	decoder := yaml.NewDecoder(bytes.NewReader(yamlData))

	for {
		var app Application
		err := decoder.Decode(&app)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode yaml document: %w", err)
		}

		if app.ApiVersion == "argoproj.io/v1alpha1" && app.Kind == "Application" {
			apps = append(apps, app)
		}
	}

	return apps, nil
}
