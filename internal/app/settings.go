package app

type Settings struct {
	Patch              string `json:"patch"`
	PatchAdditionsMode string `json:"patch_additions_mode"`
	PatchAdditions     int    `json:"patch_additions"`
	LeagueTierPreset   string `json:"league_tier_preset"`
	ApplyItems         bool   `json:"apply_items"`
	ApplyRunes         bool   `json:"apply_runes"`
	ApplySpells        bool   `json:"apply_spells"`
	KeepFlash          bool   `json:"keep_flash"`
	DryRun             bool   `json:"dry_run"`
	LCUEnabled         bool   `json:"lcu_enabled"`
}
