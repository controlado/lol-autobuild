package coachless

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-resty/resty/v2"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type Client struct {
	baseURL string
	http    *resty.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	rc := resty.New().
		SetTimeout(timeout).
		SetRetryCount(0)

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    rc,
	}
}

type refreshResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (ports.TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return ports.TokenPair{}, errors.New("refresh token is required")
	}

	var (
		endpoint = "/api/Auth/refresh"
		reqBody  = map[string]string{"refreshToken": refreshToken}
		out      refreshResponse
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, "", reqBody, &out); err != nil {
		return ports.TokenPair{}, err
	}

	return ports.TokenPair{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
	}, nil
}

func (c *Client) GetPatches(ctx context.Context, accessToken string) ([]ports.PatchInfo, error) {
	var (
		endpoint = "/api/ChampionWinprob/GetPatches"
		out      []ports.PatchInfo
	)

	if err := c.doJSON(ctx, http.MethodGet, endpoint, accessToken, nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) GetKeystoneData(ctx context.Context, accessToken string, req ports.KeystoneRequest) ([]ports.KeystoneStat, error) {
	var (
		endpoint = "/api/Rune/GetKeystoneData"
		out      []ports.KeystoneStat
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, accessToken, req, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) GetSecondaryTreePlaycount(ctx context.Context, accessToken string, req ports.SecondaryTreePlaycountRequest) ([]ports.RuneTreePlaycount, error) {
	var (
		endpoint = "/api/Rune/GetSecondaryTreePlaycount"
		out      []ports.RuneTreePlaycount
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, accessToken, req, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) GetRuneStatsForKeystoneAndTree(ctx context.Context, accessToken string, req ports.RuneStatsRequest) (ports.RuneStatsByRow, error) {
	var (
		endpoint = "/api/Rune/GetRunesForKeystoneAndTree"
		out      ports.RuneStatsByRow
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, accessToken, req, &out); err != nil {
		return ports.RuneStatsByRow{}, err
	}

	return out, nil
}

func (c *Client) GetShardStatsForKeystoneAndTree(ctx context.Context, accessToken string, req ports.ShardStatsRequest) (ports.ShardStats, error) {
	var (
		endpoint = "/api/Rune/GetShardsForKeystoneAndTree"
		out      ports.ShardStats
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, accessToken, req, &out); err != nil {
		return ports.ShardStats{}, err
	}

	return out, nil
}

func (c *Client) GetSummonerSpellStats(ctx context.Context, accessToken string, req ports.SummonerSpellStatsRequest) ([]ports.SummonerSpellStat, error) {
	var (
		out      []ports.SummonerSpellStat
		endpoint = "/api/ChampionWinprob/GetGlobalSummonerSpellStatistics"
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, accessToken, req, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) GetItemStats(ctx context.Context, accessToken string, req ports.ItemStatsRequest) ([]ports.ItemStat, error) {
	var (
		out      []ports.ItemStat
		endpoint = "/api/ChampionWinprob/GetGlobalItemStatistics"
	)

	if err := c.doJSON(ctx, http.MethodPost, endpoint, accessToken, req, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) doJSON(ctx context.Context, method, path, accessToken string, reqBody any, out any) error {
	operation := func() error {
		req := c.http.R().
			SetContext(ctx).
			SetHeader("Accept", "application/json, text/plain, */*").
			SetHeader("Content-Type", "application/json")

		if strings.TrimSpace(accessToken) != "" {
			req.SetAuthToken(accessToken)
		}

		if reqBody != nil {
			req.SetBody(reqBody)
		}

		resp, err := req.Execute(method, c.baseURL+path)
		if err != nil {
			return err
		}

		if resp.StatusCode() >= http.StatusInternalServerError {
			return fmt.Errorf("server error %d: %s", resp.StatusCode(), string(resp.Body()))
		}

		if resp.StatusCode() >= http.StatusBadRequest {
			return backoff.Permanent(fmt.Errorf("request failed %d: %s", resp.StatusCode(), string(resp.Body())))
		}

		if out != nil {
			if err := c.http.JSONUnmarshal(resp.Body(), out); err != nil {
				return backoff.Permanent(fmt.Errorf("decode response: %w", err))
			}
		}

		return nil
	}

	b := backoff.WithContext(backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(200*time.Millisecond),
		backoff.WithMaxElapsedTime(2*time.Second),
	), ctx)

	if err := backoff.Retry(operation, b); err != nil {
		return fmt.Errorf("coachless %s %s: %w", method, path, err)
	}

	return nil
}
