package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
