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
		want UserMessage
	}{
		{
			name: "nil error",
			err:  nil,
			want: UserMessage{},
		},
		{
			name: "lcu off",
			err:  fmt.Errorf("sync: %w", lcu.ErrNotConfigured),
			want: UserMessage{Code: MessageCodeLCUOff, Text: "LCU is off."},
		},
		{
			name: "lockfile missing",
			err:  fmt.Errorf("sync: %w", lcu.ErrLockfileNotFound),
			want: UserMessage{Code: MessageCodeLCULockfileNotFound, Text: "League Client is not open."},
		},
		{
			name: "champ select unavailable",
			err:  fmt.Errorf("sync: %w", lcu.ErrChampSelectUnavailable),
			want: UserMessage{Code: MessageCodeLCUChampSelectUnavailable, Text: "Champ select is not ready."},
		},
		{
			name: "champion not selected",
			err:  fmt.Errorf("sync: %w", lcu.ErrChampionNotSelected),
			want: UserMessage{Code: MessageCodeLCUChampionNotSelected, Text: "Select a champion first."},
		},
		{
			name: "coachless auth not implemented",
			err:  fmt.Errorf("auth: %w", auth.ErrNotImplemented),
			want: UserMessage{Code: MessageCodeCoachlessLoginMissing, Text: "Coachless login is missing."},
		},
		{
			name: "coachless access token error message",
			err:  errors.New("provider: unable to acquire valid access token after refresh"),
			want: UserMessage{Code: MessageCodeCoachlessLoginMissing, Text: "Coachless login is missing."},
		},
		{
			name: "fallback to raw error",
			err:  errors.New("unexpected failure"),
			want: UserMessage{Text: "unexpected failure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := userMessageFromErr(tt.err); got != tt.want {
				t.Fatalf("userMessageFromErr(%v) = %+v, want %+v", tt.err, got, tt.want)
			}

			if got := userMessageFromErr(tt.err); got.Text != tt.want.Text {
				t.Fatalf("messageFromErr(%v) = %q, want %q", tt.err, got, tt.want.Text)
			}
		})
	}
}
