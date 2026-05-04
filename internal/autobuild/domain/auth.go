package domain

import "time"

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type TokenClaims struct {
	ExpiresAt  time.Time
	Subscribed bool
}

func (tc TokenClaims) IsSubscribed() bool {
	return tc.Subscribed
}
