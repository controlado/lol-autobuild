package runes

import "github.com/controlado/lol-autobuild/internal/ports"

func StyleForKeystone(keystoneID int) (int, bool) {
	switch keystoneID {
	case ports.RuneKeystonePressTheAttack, ports.RuneKeystoneLethalTempo, ports.RuneKeystoneFleetFootwork, ports.RuneKeystoneConqueror:
		return ports.RuneStylePrecision, true
	case ports.RuneKeystoneElectrocute, ports.RuneKeystoneDarkHarvest, ports.RuneKeystoneHailOfBlades:
		return ports.RuneStyleDomination, true
	case ports.RuneKeystoneSummonAery, ports.RuneKeystoneArcaneComet, ports.RuneKeystoneStormraidersSurge, ports.RuneKeystoneDeathfireTouch:
		return ports.RuneStyleSorcery, true
	case ports.RuneKeystoneGraspOfTheUndying, ports.RuneKeystoneAftershock, ports.RuneKeystoneGuardian:
		return ports.RuneStyleResolve, true
	case ports.RuneKeystoneGlacialAugment, ports.RuneKeystoneUnsealedSpellbook, ports.RuneKeystoneFirstStrike:
		return ports.RuneStyleInspiration, true
	default:
		return 0, false
	}
}

func IsStyle(styleID int) bool {
	switch styleID {
	case ports.RuneStylePrecision, ports.RuneStyleDomination, ports.RuneStyleSorcery, ports.RuneStyleInspiration, ports.RuneStyleResolve:
		return true
	default:
		return false
	}
}

func RecommendedSecondaryStyle(playcounts []ports.RuneTreePlaycount, primaryStyleID int) (int, bool) {
	var (
		best ports.RuneTreePlaycount
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
