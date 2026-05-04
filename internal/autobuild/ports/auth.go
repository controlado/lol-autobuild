package ports

import (
	"context"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type TokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
	Claims(ctx context.Context) (domain.TokenClaims, error)
}
