package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/controlado/lol-autobuild/internal/auth"
	"github.com/controlado/lol-autobuild/internal/lcu"
)

func TestMessageFromErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "lcu off",
			err:  fmt.Errorf("sync: %w", lcu.ErrNotConfigured),
			want: "LCU is off.",
		},
		{
			name: "lockfile missing",
			err:  fmt.Errorf("sync: %w", lcu.ErrLockfileNotFound),
			want: "League Client is not open.",
		},
		{
			name: "champ select unavailable",
			err:  fmt.Errorf("sync: %w", lcu.ErrChampSelectUnavailable),
			want: "Champ select is not ready.",
		},
		{
			name: "champion not selected",
			err:  fmt.Errorf("sync: %w", lcu.ErrChampionNotSelected),
			want: "Select a champion first.",
		},
		{
			name: "coachless auth not implemented",
			err:  fmt.Errorf("auth: %w", auth.ErrNotImplemented),
			want: "Coachless login is missing.",
		},
		{
			name: "coachless access token error message",
			err:  errors.New("provider: unable to acquire valid access token after refresh"),
			want: "Coachless login is missing.",
		},
		{
			name: "fallback to raw error",
			err:  errors.New("unexpected failure"),
			want: "unexpected failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := messageFromErr(tt.err); got != tt.want {
				t.Fatalf("messageFromErr(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
