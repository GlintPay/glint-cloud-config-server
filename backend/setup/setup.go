package setup

import (
	"context"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/backend/file"
	"github.com/GlintPay/gccs/backend/git"
	"github.com/GlintPay/gccs/config"
	"github.com/rs/zerolog/log"
)

func Init(ctx context.Context, appConfig config.ApplicationConfiguration) (backend.Backends, error) {
	var backends backend.Backends

	if appConfig.Git.Disabled {
		log.Info().Msg("Git backend is disabled")
	} else {
		log.Info().Msg("Enabling Git backend")
		backends = append(backends, &git.Backend{EnableTrace: appConfig.Tracing.Enabled})
	}

	if appConfig.File.Disabled {
		log.Info().Msg("File backend is disabled")
	} else {
		log.Info().Msg("Enabling File backend")
		backends = append(backends, &file.Backend{})
	}

	for _, each := range backends {
		if backendErr := each.Init(ctx, appConfig); backendErr != nil {
			return nil, backendErr
		}
	}

	return backends, nil
}
