package runes

import "github.com/controlado/lol-autobuild/internal/autobuild/domain"

func StyleForKeystone(keystoneID int) (int, bool) {
	switch keystoneID {
	case domain.RuneKeystonePressTheAttack, domain.RuneKeystoneLethalTempo, domain.RuneKeystoneFleetFootwork, domain.RuneKeystoneConqueror:
		return domain.RuneStylePrecision, true
	case domain.RuneKeystoneElectrocute, domain.RuneKeystoneDarkHarvest, domain.RuneKeystoneHailOfBlades:
		return domain.RuneStyleDomination, true
	case domain.RuneKeystoneSummonAery, domain.RuneKeystoneArcaneComet, domain.RuneKeystoneStormraidersSurge, domain.RuneKeystoneDeathfireTouch:
		return domain.RuneStyleSorcery, true
	case domain.RuneKeystoneGraspOfTheUndying, domain.RuneKeystoneAftershock, domain.RuneKeystoneGuardian:
		return domain.RuneStyleResolve, true
	case domain.RuneKeystoneGlacialAugment, domain.RuneKeystoneUnsealedSpellbook, domain.RuneKeystoneFirstStrike:
		return domain.RuneStyleInspiration, true
	default:
		return 0, false
	}
}

func IsStyle(styleID int) bool {
	switch styleID {
	case domain.RuneStylePrecision, domain.RuneStyleDomination, domain.RuneStyleSorcery, domain.RuneStyleInspiration, domain.RuneStyleResolve:
		return true
	default:
		return false
	}
}

func RecommendedSecondaryStyle(playcounts []domain.RuneTreePlaycount, primaryStyleID int) (int, bool) {
	var (
		best domain.RuneTreePlaycount
		ok   bool
	)

	for _, stat := range playcounts {
		if stat.Tree == primaryStyleID || !IsStyle(stat.Tree) {
			continue
		}
		if !ok || stat.Occurrence > best.Occurrence || stat.Occurrence == best.Occurrence && stat.Tree < best.Tree {
			best = stat
			ok = true
		}
	}

	if !ok {
		return 0, false
	}
	return best.Tree, true
}
