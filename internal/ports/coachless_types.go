package ports

type CoachlessRoleCode int

const (
	CoachlessRoleTop CoachlessRoleCode = iota
	CoachlessRoleJungle
	CoachlessRoleMid
	CoachlessRoleADC
	CoachlessRoleSupport
)

type CoachlessLeagueTier int

const (
	CoachlessLeagueTierFive  CoachlessLeagueTier = 5
	CoachlessLeagueTierSix   CoachlessLeagueTier = 6
	CoachlessLeagueTierSeven CoachlessLeagueTier = 7
)

type CoachlessItemType int

const (
	CoachlessItemTypeDefault CoachlessItemType = 6
)

type CoachlessPatchAdditions int

const (
	CoachlessPatchAdditionsDefault CoachlessPatchAdditions = 2
)
