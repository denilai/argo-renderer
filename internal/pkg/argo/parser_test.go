package argo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseApplications(t *testing.T) {
	t.Run("should parse a single valid application with all fields", func(t *testing.T) {
		yamlData := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  labels:
    env: dev
    instance: inf1
  annotations:
    rawRepository: "https://my.repo/project.git"
    rawPath: "charts/my-app"
spec:
  source:
    targetRevision: main
    plugin:
      env:
        - name: WERF_SET_KEY
          value: "global.key=value"
`
		apps, err := ParseApplications([]byte(yamlData))
		require.NoError(t, err)
		require.Len(t, apps, 1)

		app := apps[0]
		require.Equal(t, "my-app", app.Metadata.Name)
		require.Equal(t, "dev", app.Metadata.Labels["env"])
		require.Equal(t, "inf1", app.Metadata.Labels["instance"])
		require.Equal(t, "https://my.repo/project.git", app.Metadata.Annotations["rawRepository"])
		require.Equal(t, "charts/my-app", app.Metadata.Annotations["rawPath"])
		require.Equal(t, "main", app.Spec.Source.TargetRevision)
		require.Len(t, app.Spec.Source.Plugin.Env, 1)
		require.Equal(t, "WERF_SET_KEY", app.Spec.Source.Plugin.Env[0].Name)
	})

	t.Run("should ignore other kubernetes resources in a multi-document yaml", func(t *testing.T) {
		yamlData := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: the-real-app
`
		apps, err := ParseApplications([]byte(yamlData))
		require.NoError(t, err)
		require.Len(t, apps, 1)
		require.Equal(t, "the-real-app", apps[0].Metadata.Name)
	})

	t.Run("should return error on malformed yaml", func(t *testing.T) {
		yamlData := `apiVersion: v1: kind: Broken`
		_, err := ParseApplications([]byte(yamlData))
		require.Error(t, err)
	})
}
