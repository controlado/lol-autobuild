package lcu

import (
	"fmt"
	"strings"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

const (
	managedNamePrefix     = "[autobuild]"
	legacyAutoBuildPrefix = "AutoBuild "
)

func managedResourceTitle(position domain.Position, championID int, championName string) string {
	championLabel := strings.TrimSpace(championName)
	if championLabel == "" {
		championLabel = fmt.Sprint(championID)
	}

	positionLabel := strings.TrimSpace(position.String())
	if positionLabel == "" {
		return managedNamePrefix + " " + championLabel
	}

	return fmt.Sprintf("%s [%s] %s", managedNamePrefix, positionLabel, championLabel)
}
