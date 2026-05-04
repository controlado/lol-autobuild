package app

import (
	"errors"
	"testing"
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
			name: "raw error",
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
