package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
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
	LoginURL       string
	AcquireTimeout time.Duration
}

func (s BrowserSource) Acquire(ctx context.Context) (ports.TokenPair, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	ctx, cancel := context.WithTimeout(browserCtx, s.AcquireTimeout)
	defer cancel()

	tokenPairChannel := make(chan ports.TokenPair, 1)
	onAuthResponse := func(e *network.EventResponseReceived) {
		var (
			body          []byte
			bodyExtractor = func(ctx context.Context) (handlerErr error) {
				body, handlerErr = network.GetResponseBody(e.RequestID).Do(ctx)
				return
			}
			action = chromedp.ActionFunc(bodyExtractor)
		)
		if err := chromedp.Run(ctx, action); err != nil {
			return
		}

		pair, ok := s.tokenPairFromRawBody(body)
		if !ok {
			return
		}

		select {
		case tokenPairChannel <- pair:
		default:
		}
	}

	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Response == nil || !strings.Contains(e.Response.URL, "/api/Auth/login") {
				return
			}
			go onAuthResponse(e)
		}
	})

	if err := chromedp.Run(
		ctx,
		network.Enable(),
		chromedp.Navigate(s.LoginURL),
	); err != nil {
		return ports.TokenPair{}, fmt.Errorf("running browser: %w", err)
	}

	select {
	case pair := <-tokenPairChannel:
		return pair, nil
	case <-ctx.Done():
		return ports.TokenPair{}, ctx.Err()
	}
}

func (s BrowserSource) tokenPairFromRawBody(raw []byte) (ports.TokenPair, bool) {
	if len(raw) < 1 {
		return ports.TokenPair{}, false
	}

	var dto = struct {
		ActionResult int    `json:"actionResult"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}{}
	if err := json.Unmarshal(raw, &dto); err != nil {
		return ports.TokenPair{}, false
	}

	if dto.AccessToken == "" || dto.RefreshToken == "" {
		return ports.TokenPair{}, false
	}

	return ports.TokenPair{
		AccessToken:  dto.AccessToken,
		RefreshToken: dto.RefreshToken,
	}, true
}

type EnvManualSource struct{}

func (EnvManualSource) Acquire(_ context.Context) (ports.TokenPair, error) {
	var (
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
