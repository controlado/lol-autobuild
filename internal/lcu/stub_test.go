package lcu

import (
	"context"

	"github.com/controlado/lol-autobuild/internal/ports"
)

type StubClient struct {
	DetectedSelection  ports.DetectedSelection
	DetectErr          error
	ItemSetCalls       []ports.ApplyItemSetRequest
	RunePageCalls      []ports.ApplyRunePageRequest
	SummonerSpellCalls []ports.ApplySummonerSpellsRequest
	WatchEventsCalls   int
	WatchEventsErr     error
	ItemSetErr         error
	RunePageErr        error
	SummonerSpellsErr  error
}

func (c *StubClient) DetectSelection(ctx context.Context) (ports.DetectedSelection, error) {
	_ = ctx
	if c.DetectErr != nil {
		return ports.DetectedSelection{}, c.DetectErr
	}

	return c.DetectedSelection, nil
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

func (c *StubClient) WatchEvents(ctx context.Context, out chan<- ports.LCUEvent) error {
	_ = ctx
	_ = out
	c.WatchEventsCalls++
	return c.WatchEventsErr
}
