package app

type Settings struct {
	Patch       string `json:"patch"`
	ApplyItems  bool   `json:"apply_items"`
	ApplyRunes  bool   `json:"apply_runes"`
	ApplySpells bool   `json:"apply_spells"`
	KeepFlash   bool   `json:"keep_flash"`
	DryRun      bool   `json:"dry_run"`
	LCUEnabled  bool   `json:"lcu_enabled"`
}
