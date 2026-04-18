package position

import (
	"fmt"
	"strings"
)

type Position string

const (
	Top     Position = "top"
	Jungle  Position = "jungle"
	Mid     Position = "mid"
	ADC     Position = "adc"
	Support Position = "support"
)

func FromRaw(r string) (Position, error) {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "top", "0":
		return Top, nil
	case "jungle", "1":
		return Jungle, nil
	case "mid", "middle", "2":
		return Mid, nil
	case "adc", "bot", "bottom", "3":
		return ADC, nil
	case "support", "sup", "utility", "4":
		return Support, nil
	case "", "fill", "unselected":
		return "", ErrNotAssigned
	default:
		return "", fmt.Errorf("%w: position %q", ErrUnknown, r)
	}
}

func (p Position) Code() int {
	switch p {
	case Top:
		return 0
	case Jungle:
		return 1
	case Mid:
		return 2
	case ADC:
		return 3
	case Support:
		return 4
	default:
		return -1
	}
}

func (p Position) String() string {
	return string(p)
}

func (p Position) IsSupport() bool {
	return p == Support
}

func (p Position) IsValid() bool {
	switch p {
	case Top, Jungle, Mid, ADC, Support:
		return true
	default:
		return false
	}
}
