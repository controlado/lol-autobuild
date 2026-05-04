package ports

import (
	"context"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type LCUClient interface {
	DetectSelection(ctx context.Context) (domain.DetectedSelection, error)
	ApplyItemSet(ctx context.Context, req domain.ApplyItemSetRequest) error
	ApplyRunePage(ctx context.Context, req domain.ApplyRunePageRequest) error
	ApplySummonerSpells(ctx context.Context, req domain.ApplySummonerSpellsRequest) error
	WatchEventsWithNotices(ctx context.Context, out chan<- domain.LCUEvent, notices chan<- domain.LCUWatchNotice) error
}
