package app

import (
	"testing"
	"werf-argo-renderer/internal/pkg/argo"

	"github.com/stretchr/testify/require"
)

func TestProcessPluginEnv(t *testing.T) {
	t.Run("should correctly parse set values and sort values files", func(t *testing.T) {
		envVars := []argo.EnvVar{
			{Name: "WERF_VALUES_10", Value: "values10.yaml"},
			{Name: "WERF_SET_IMAGE", Value: "global.image.tag=v1.2.3"},
			{Name: "WERF_VALUES_1", Value: "values1.yaml"},
			{Name: "WERF_VALUES_2", Value: "values2.yaml"},
			{Name: "SOME_OTHER_VAR", Value: "some_value"},
			{Name: "WERF_SET_INVALID", Value: "no_equals_sign"},
		}

		setValues, valuesFiles := processPluginEnv(envVars)

		// Проверяем --set
		require.Len(t, setValues, 1)
		require.Equal(t, "v1.2.3", setValues["global.image.tag"])
		_, exists := setValues["WERF_SET_INVALID"]
		require.False(t, exists)

		// Проверяем --values и их порядок
		require.Len(t, valuesFiles, 3)
		require.Equal(t, "values1.yaml", valuesFiles[0])
		require.Equal(t, "values2.yaml", valuesFiles[1])
		require.Equal(t, "values10.yaml", valuesFiles[2])
	})
}

func TestConvertHTTPtoSSH(t *testing.T) {
	tests := []struct {
		name     string
		inputURL string
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "standard https url",
			inputURL: "https://gitlab.com/my-org/my-repo.git",
			wantURL:  "git@gitlab.com:my-org/my-repo.git",
			wantErr:  false,
		},
		{
			name:     "url is already ssh",
			inputURL: "git@github.com:another-org/another-repo.git",
			wantURL:  "git@github.com:another-org/another-repo.git",
			wantErr:  false,
		},
		{
			name:     "http url without .git suffix",
			inputURL: "http://gitserver.internal/project/repo",
			wantURL:  "git@gitserver.internal:project/repo",
			wantErr:  false,
		},
		{
			name:     "malformed url",
			inputURL: "://some-broken-url",
			wantURL:  "",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, err := convertHTTPtoSSH(tt.inputURL)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantURL, gotURL)
			}
		})
	}
}
