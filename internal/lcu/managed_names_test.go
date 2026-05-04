package lcu

import (
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestManagedResourceTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		position     domain.Position
		championID   int
		championName string
		want         string
	}{
		{
			name:         "uses champion name",
			position:     domain.Support,
			championID:   240,
			championName: "Kled",
			want:         "[autobuild] [support] Kled",
		},
		{
			name:       "falls back to champion id",
			position:   domain.Top,
			championID: 240,
			want:       "[autobuild] [top] 240",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := managedResourceTitle(tt.position, tt.championID, tt.championName)
			if got != tt.want {
				t.Fatalf("managedResourceTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsManagedRunePageRecognizesNewAndLegacyNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		page runePage
		want bool
	}{
		{name: "new", page: runePage{Name: "[autobuild] [top] Kled"}, want: true},
		{name: "legacy", page: runePage{Name: "AutoBuild 240 top"}, want: true},
		{name: "user", page: runePage{Name: "My Page"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isManagedRunePage(tt.page); got != tt.want {
				t.Fatalf("isManagedRunePage() = %t, want %t", got, tt.want)
			}
		})
	}
}
