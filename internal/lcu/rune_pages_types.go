package lcu

type runePage struct {
	ID              int    `json:"id,omitempty"`
	Current         bool   `json:"current"`
	IsActive        bool   `json:"isActive,omitempty"`
	IsValid         bool   `json:"isValid,omitempty"`
	IsEditable      bool   `json:"isEditable,omitempty"`
	IsDeletable     bool   `json:"isDeletable,omitempty"`
	IsTemporary     bool   `json:"isTemporary,omitempty"`
	Name            string `json:"name"`
	Order           int    `json:"order,omitempty"`
	PrimaryStyleID  int    `json:"primaryStyleId"`
	SubStyleID      int    `json:"subStyleId"`
	SelectedPerkIDs []int  `json:"selectedPerkIds"`
}

type runePageCreateRequest struct {
	Name            string `json:"name"`
	PrimaryStyleID  int    `json:"primaryStyleId"`
	SubStyleID      int    `json:"subStyleId"`
	SelectedPerkIDs []int  `json:"selectedPerkIds"`
	Current         bool   `json:"current"`
}
