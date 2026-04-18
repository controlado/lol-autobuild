package lcu

import "strings"

type connectionInfo struct {
	Port     int
	Password string
	Protocol string
}

type connectionCandidate struct {
	source  string
	resolve func() (connectionInfo, error)
}

func (cc connectionCandidate) label() string {
	source := strings.TrimSpace(cc.source)

	if source == "" {
		return "unknown"
	}

	return source
}
