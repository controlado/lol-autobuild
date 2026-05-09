package domain

type RunePage struct {
	PrimaryStyleID  int
	SubStyleID      int
	SelectedPerkIDs []int
}

const (
	RuneStylePrecision   = 8000
	RuneStyleDomination  = 8100
	RuneStyleSorcery     = 8200
	RuneStyleInspiration = 8300
	RuneStyleResolve     = 8400
)

const (
	RuneKeystonePressTheAttack    = 8005
	RuneKeystoneLethalTempo       = 8008
	RuneKeystoneFleetFootwork     = 8021
	RuneKeystoneConqueror         = 8010
	RuneKeystoneElectrocute       = 8112
	RuneKeystoneDarkHarvest       = 8128
	RuneKeystoneHailOfBlades      = 9923
	RuneKeystoneSummonAery        = 8214
	RuneKeystoneArcaneComet       = 8229
	RuneKeystoneStormraidersSurge = 8230
	RuneKeystoneDeathfireTouch    = 8992
	RuneKeystoneGraspOfTheUndying = 8437
	RuneKeystoneAftershock        = 8439
	RuneKeystoneGuardian          = 8465
	RuneKeystoneGlacialAugment    = 8351
	RuneKeystoneUnsealedSpellbook = 8360
	RuneKeystoneFirstStrike       = 8369
)
