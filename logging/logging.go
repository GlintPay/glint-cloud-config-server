package logging

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"io"
)

func Setup(w io.Writer) {
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000"
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	log.Logger = zerolog.New(w).With().Timestamp().Caller().Logger()
}
