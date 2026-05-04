package ports

import (
	"context"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type CoachlessClient interface {
	GetPatches(ctx context.Context, accessToken string) ([]domain.PatchInfo, error)
	GetKeystoneData(ctx context.Context, accessToken string, req domain.KeystoneRequest) ([]domain.KeystoneStat, error)
	GetSecondaryTreePlaycount(ctx context.Context, accessToken string, req domain.SecondaryTreePlaycountRequest) ([]domain.RuneTreePlaycount, error)
	GetRuneStatsForKeystoneAndTree(ctx context.Context, accessToken string, req domain.RuneStatsRequest) (domain.RuneStatsByRow, error)
	GetShardStatsForKeystoneAndTree(ctx context.Context, accessToken string, req domain.ShardStatsRequest) (domain.ShardStats, error)
	GetSummonerSpellStats(ctx context.Context, accessToken string, req domain.SummonerSpellStatsRequest) ([]domain.SummonerSpellStat, error)
	GetItemStats(ctx context.Context, accessToken string, req domain.ItemStatsRequest) ([]domain.ItemStat, error)
}
