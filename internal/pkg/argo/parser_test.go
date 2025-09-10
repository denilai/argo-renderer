package argo

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// // TestParseApplications - это основной набор тестов для парсера.
// func TestParseApplications(t *testing.T) {
// 	// --- Тесты на базовый парсинг и фильтрацию ---
// 	t.Run("should ignore non-application resources", func(t *testing.T) {
// 		yamlData := `
// apiVersion: v1
// kind: ConfigMap
// metadata:
//   name: my-config
// ---
// apiVersion: argoproj.io/v1alpha1
// kind: Application
// metadata:
//   name: the-real-app
//   annotations:
//     rawRepository: "repo"
// spec:
//   source:
//     targetRevision: main
// `
// 		apps, err := ParseApplications([]byte(yamlData))
// 		require.NoError(t, err)
// 		require.Len(t, apps, 1)
// 		require.Equal(t, "the-real-app", apps[0].Name)
// 	})

// 	t.Run("should return error on malformed yaml", func(t *testing.T) {
// 		yamlData := `apiVersion: v1: kind: Broken`
// 		_, err := ParseApplications([]byte(yamlData))
// 		require.Error(t, err)
// 	})

// 	t.Run("should return error on missing repo url", func(t *testing.T) {
// 		yamlData := `
// apiVersion: argoproj.io/v1alpha1
// kind: Application
// metadata:
//   name: test-app
// spec:
//   source:
//     targetRevision: main
// `
// 		_, err := ParseApplications([]byte(yamlData))
// 		require.Error(t, err)
// 		require.Contains(t, err.Error(), "both 'rawRepository' annotation and 'spec.source.repoURL' are empty")
// 	})
// }

// func TestInstanceAndEnvResolution(t *testing.T) {
// 	baseYAML := `
// apiVersion: argoproj.io/v1alpha1
// kind: Application
// metadata:
//   name: test-app
//   annotations:
//     rawRepository: "repo"
// %s
// spec:
//   source:
//     targetRevision: main
// %s
// `
// 	testCases := []struct {
// 		name             string
// 		labelsSection    string
// 		pluginSection    string
// 		expectedInstance string
// 		expectedEnv      string
// 		expectError      bool
// 		errorContains    string
// 	}{
// 		{
// 			name:             "values from labels only",
// 			labelsSection:    `  labels: { instance: "from-label", env: "dev-label" }`,
// 			pluginSection:    ``,
// 			expectedInstance: "from-label",
// 			expectedEnv:      "dev-label",
// 			expectError:      false,
// 		},
// 		{
// 			name:          "values from plugin.env only",
// 			labelsSection: ``,
// 			pluginSection: `
//     plugin:
//       env:
//         - name: WERF_SET_INSTANCE
//           value: "global.instance=from-plugin"
//         - name: WERF_SET_ENV
//           value: "global.env=dev-plugin"`,
// 			expectedInstance: "from-plugin",
// 			expectedEnv:      "dev-plugin",
// 			expectError:      false,
// 		},
// 		{
// 			name:          "labels have priority when both sources match",
// 			labelsSection: `  labels: { instance: "same", env: "same-dev" }`,
// 			pluginSection: `
//     plugin:
//       env:
//         - name: WERF_SET_INSTANCE
//           value: "global.instance=same"
//         - name: WERF_SET_ENV
//           value: "global.env=same-dev"`,
// 			expectedInstance: "same",
// 			expectedEnv:      "same-dev",
// 			expectError:      false,
// 		},
// 		{
// 			name:          "conflict when both sources differ for instance",
// 			labelsSection: `  labels: { instance: "from-label" }`,
// 			pluginSection: `
//     plugin:
//       env:
//         - name: WERF_SET_INSTANCE
//           value: "global.instance=from-plugin"`,
// 			expectError:   true,
// 			errorContains: "conflicting values for 'instance'",
// 		},
// 		{
// 			name:          "conflict when both sources differ for env",
// 			labelsSection: `  labels: { env: "from-label" }`,
// 			pluginSection: `
//     plugin:
//       env:
//         - name: WERF_SET_ENV
//           value: "global.env=from-plugin"`,
// 			expectError:   true,
// 			errorContains: "conflicting values for 'env'",
// 		},
// 		{
// 			name:             "no values provided for instance and env",
// 			labelsSection:    ``,
// 			pluginSection:    ``,
// 			expectedInstance: "",
// 			expectedEnv:      "",
// 			expectError:      false,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			yamlData := fmt.Sprintf(baseYAML, tc.labelsSection, tc.pluginSection)
// 			fmt.Printf("YAML Data:\n%s\n", yamlData) // Debug output

// 			apps, err := ParseApplications([]byte(yamlData))

//				if tc.expectError {
//					require.Error(t, err)
//					require.Contains(t, err.Error(), tc.errorContains)
//				} else {
//					require.NoError(t, err)
//					require.Len(t, apps, 1)
//					require.Equal(t, tc.expectedInstance, apps[0].Instance)
//					require.Equal(t, tc.expectedEnv, apps[0].Env)
//				}
//			})
//		}
//	}

// newTestRawApp создает базовую, минимально валидную структуру rawApplication для тестов.
func newTestRawApp() rawApplication {
	var app rawApplication
	app.ApiVersion = "argoproj.io/v1alpha1"
	app.Kind = "Application"
	app.Metadata.Name = "test-app"
	app.Spec.Source.TargetRevision = "main"
	app.Metadata.Annotations = map[string]string{"rawRepository": "https://test.repo"}
	return app
}

func TestRepoAndPathResolution(t *testing.T) {
	testCases := []struct {
		name            string
		modifier        func(*rawApplication)
		expectedRepoURL string
		expectedPath    string
		expectError     bool
		errorContains   string
	}{
		{
			name: "primary source: values from annotations",
			modifier: func(app *rawApplication) {
				app.Metadata.Annotations["rawRepository"] = "https://anno.repo"
				app.Metadata.Annotations["rawPath"] = "anno/path"
				app.Spec.Source.RepoURL = "https://spec.repo" // Добавляем, чтобы проверить приоритет
			},
			expectedRepoURL: "https://anno.repo",
			expectedPath:    "anno/path",
		},
		{
			name: "fallback source: values from spec.source",
			modifier: func(app *rawApplication) {
				app.Metadata.Annotations = map[string]string{} // Убираем rawRepository по умолчанию
				app.Spec.Source.RepoURL = "https://spec.repo"
				app.Spec.Source.Path = "spec/path"
			},
			expectedRepoURL: "https://spec.repo",
			expectedPath:    "spec/path",
		},
		{
			name: "path fallback: rawPath and spec.source.path are missing",
			modifier: func(app *rawApplication) {
				// app.Path по умолчанию пуст
			},
			expectedRepoURL: "https://test.repo",
			expectedPath:    ".",
		},
		{
			name: "error: all repo sources are missing",
			modifier: func(app *rawApplication) {
				app.Metadata.Annotations = map[string]string{}
				app.Spec.Source.RepoURL = ""
			},
			expectError:   true,
			errorContains: "both 'rawRepository' annotation and 'spec.source.repoURL' are empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawApp := newTestRawApp()
			tc.modifier(&rawApp)

			yamlData, err := yaml.Marshal(rawApp)
			require.NoError(t, err)

			apps, err := ParseApplications(yamlData)
			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
				require.Len(t, apps, 1)
				require.Equal(t, tc.expectedRepoURL, apps[0].RepoURL)
				require.Equal(t, tc.expectedPath, apps[0].Path)
			}
		})
	}
}
