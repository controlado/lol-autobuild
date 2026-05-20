package autobuild

import (
	"fmt"
	"slices"
)

type CoachlessRegion struct {
	ID    int
	Label string
}

const (
	CoachlessRegionBR   = 0
	CoachlessRegionEUNE = 1
	CoachlessRegionEUW  = 2
	CoachlessRegionJP   = 3
	CoachlessRegionKR   = 4
	CoachlessRegionLAN  = 5
	CoachlessRegionLAS  = 6
	CoachlessRegionME   = 7
	CoachlessRegionNA   = 8
	CoachlessRegionOCE  = 9
	CoachlessRegionRU   = 11
	CoachlessRegionSG   = 12
	CoachlessRegionTR   = 14
	CoachlessRegionTW   = 15
	CoachlessRegionVN   = 16
)

var coachlessRegions = []CoachlessRegion{
	{ID: CoachlessRegionBR, Label: "BR"},
	{ID: CoachlessRegionEUNE, Label: "EUNE"},
	{ID: CoachlessRegionEUW, Label: "EUW"},
	{ID: CoachlessRegionJP, Label: "JP"},
	{ID: CoachlessRegionKR, Label: "KR"},
	{ID: CoachlessRegionLAN, Label: "LAN"},
	{ID: CoachlessRegionLAS, Label: "LAS"},
	{ID: CoachlessRegionME, Label: "ME"},
	{ID: CoachlessRegionNA, Label: "NA"},
	{ID: CoachlessRegionOCE, Label: "OCE"},
	{ID: CoachlessRegionRU, Label: "RU"},
	{ID: CoachlessRegionSG, Label: "SG"},
	{ID: CoachlessRegionTR, Label: "TR"},
	{ID: CoachlessRegionTW, Label: "TW"},
	{ID: CoachlessRegionVN, Label: "VN"},
}

func NormalizeCoachlessRegions(regions []int) ([]int, error) {
	if len(regions) == 0 {
		return nil, nil
	}

	allowed := CoachlessRegionIDs()
	seen := make(map[int]struct{}, len(regions))
	for _, region := range regions {
		if !slices.Contains(allowed, region) {
			return nil, fmt.Errorf("coachless region %d is invalid", region)
		}
		seen[region] = struct{}{}
	}

	if len(seen) == len(coachlessRegions) {
		return nil, nil
	}

	out := make([]int, 0, len(seen))
	for _, region := range coachlessRegions {
		if _, ok := seen[region.ID]; ok {
			out = append(out, region.ID)
		}
	}
	return out, nil
}

func CoachlessRegions() []CoachlessRegion {
	return slices.Clone(coachlessRegions)
}

func CoachlessRegionIDs() []int {
	regions := CoachlessRegions()
	out := make([]int, 0, len(regions))
	for _, region := range regions {
		out = append(out, region.ID)
	}
	return out
}
