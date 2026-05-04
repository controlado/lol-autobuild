package ports

import "github.com/controlado/lol-autobuild/internal/autobuild/domain"

type RecommendationEngine interface {
	Recommend(input domain.RecommendationInput) domain.Recommendation
	RecommendRunePage(input domain.RunePageRecommendationInput) domain.RunePageRecommendation
}
