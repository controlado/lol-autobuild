package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/controlado/lol-autobuild/internal/ports"
)

var ErrNotImplemented = errors.New("not implemented")

type AutoSource interface {
	Acquire(ctx context.Context) (ports.TokenPair, error)
}

type ManualSource interface {
	Acquire(ctx context.Context) (ports.TokenPair, error)
}

type BrowserSource struct {
	LoginURL string
}

func (s BrowserSource) Acquire(ctx context.Context) (ports.TokenPair, error) {
	_ = chromedp.Tasks{}

	if strings.TrimSpace(s.LoginURL) == "" {
		return ports.TokenPair{}, fmt.Errorf("browser source: login URL is required")
	}

	return ports.TokenPair{}, fmt.Errorf("browser source: %w", ErrNotImplemented)
}

type EnvManualSource struct{}

func (EnvManualSource) Acquire(ctx context.Context) (ports.TokenPair, error) {
	var (
		_       = ctx
		access  = strings.TrimSpace(os.Getenv("COACHLESS_ACCESS_TOKEN"))
		refresh = strings.TrimSpace(os.Getenv("COACHLESS_REFRESH_TOKEN"))
		expRaw  = strings.TrimSpace(os.Getenv("COACHLESS_ACCESS_TOKEN_EXP"))
	)

	if access == "" {
		return ports.TokenPair{}, errors.New("manual source: COACHLESS_ACCESS_TOKEN is required")
	}

	var exp = time.Now().Add(15 * time.Minute) // default
	if expRaw != "" {
		unix, err := strconv.ParseInt(expRaw, 10, 64)
		if err != nil {
			return ports.TokenPair{}, fmt.Errorf("manual source: invalid COACHLESS_ACCESS_TOKEN_EXP: %w", err)
		}

		exp = time.Unix(unix, 0).UTC()
	}

	return ports.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    exp,
	}, nil
}
