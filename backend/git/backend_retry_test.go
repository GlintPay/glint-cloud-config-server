package git

import (
	"context"
	"errors"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/require"
)

func TestIsTransient(t *testing.T) {
	require.True(t, isTransient(errors.New("net/http: TLS handshake timeout")))
	require.False(t, isTransient(transport.ErrAuthenticationRequired))
	require.False(t, isTransient(transport.ErrRepositoryNotFound))
	require.False(t, isTransient(context.Canceled))
}
