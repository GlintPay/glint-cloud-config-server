package setup

import (
	"context"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/backend/file"
	"github.com/GlintPay/gccs/backend/git"
	"github.com/GlintPay/gccs/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []example{
		{
			name:      "default",
			appConfig: config.ApplicationConfiguration{},
			want: backend.Backends([]backend.Backend{
				&git.Backend{EnableTrace: false},
				&file.Backend{},
			}),
			wantErr: false,
		},
		{
			name:      "no-git",
			appConfig: config.ApplicationConfiguration{Git: config.GitConfig{Disabled: true}},
			want: backend.Backends([]backend.Backend{
				&file.Backend{},
			}),
			wantErr: false,
		},
		{
			name:      "no-file",
			appConfig: config.ApplicationConfiguration{File: config.FileConfig{Disabled: true}},
			want: backend.Backends([]backend.Backend{
				&git.Backend{EnableTrace: false},
			}),
			wantErr: false,
		},
		{
			name: "nothing",
			appConfig: config.ApplicationConfiguration{
				Git:  config.GitConfig{Disabled: true},
				File: config.FileConfig{Disabled: true},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := Init(context.Background(), tt.appConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

type example struct {
	name      string
	appConfig config.ApplicationConfiguration
	want      backend.Backends
	wantErr   bool
}
