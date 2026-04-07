package lcu

import (
	"context"
	"errors"
	"fmt"

	"github.com/controlado/lol-autobuild/internal/ports"
)

var ErrNotConfigured = errors.New("lcu client is not configured")

type Client struct {
	Enabled bool
}

func NewClient(enabled bool) *Client {
	return &Client{Enabled: enabled}
}

func (c *Client) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply item set: %w", ErrNotConfigured)
}

func (c *Client) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply rune page: %w", ErrNotConfigured)
}

func (c *Client) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	_ = ctx
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply summoner spells: %w", ErrNotConfigured)
}

type StubClient struct {
	ItemSetCalls       []ports.ApplyItemSetRequest
	RunePageCalls      []ports.ApplyRunePageRequest
	SummonerSpellCalls []ports.ApplySummonerSpellsRequest
	ItemSetErr         error
	RunePageErr        error
	SummonerSpellsErr  error
}

func (c *StubClient) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx
	c.ItemSetCalls = append(c.ItemSetCalls, req)
	return c.ItemSetErr
}

func (c *StubClient) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx
	c.RunePageCalls = append(c.RunePageCalls, req)
	return c.RunePageErr
}

func (c *StubClient) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	_ = ctx
	c.SummonerSpellCalls = append(c.SummonerSpellCalls, req)
	return c.SummonerSpellsErr
}
