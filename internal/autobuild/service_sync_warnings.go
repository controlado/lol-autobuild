package autobuild

import (
	"errors"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

const runePageLimitReachedWarning = "Rune page limit reached. Delete a rune page in League Client or keep an AutoBuild page available for reuse."

func runePageApplyWarning(err error) string {
	if errors.Is(err, domain.ErrRunePageLimitReached) {
		return runePageLimitReachedWarning
	}
	return "failed to apply rune page: " + err.Error()
}
