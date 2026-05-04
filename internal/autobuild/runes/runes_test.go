package runes

import (
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestStyleForKeystone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		keystone  int
		wantStyle int
		wantOK    bool
	}{
		{name: "precision", keystone: domain.RuneKeystonePressTheAttack, wantStyle: domain.RuneStylePrecision, wantOK: true},
		{name: "domination", keystone: domain.RuneKeystoneElectrocute, wantStyle: domain.RuneStyleDomination, wantOK: true},
		{name: "sorcery", keystone: domain.RuneKeystoneArcaneComet, wantStyle: domain.RuneStyleSorcery, wantOK: true},
		{name: "resolve", keystone: domain.RuneKeystoneGraspOfTheUndying, wantStyle: domain.RuneStyleResolve, wantOK: true},
		{name: "inspiration", keystone: domain.RuneKeystoneFirstStrike, wantStyle: domain.RuneStyleInspiration, wantOK: true},
		{name: "unknown", keystone: 9999, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := StyleForKeystone(tt.keystone)
			if ok != tt.wantOK || got != tt.wantStyle {
				t.Fatalf("StyleForKeystone(%d) = (%d, %v), want (%d, %v)", tt.keystone, got, ok, tt.wantStyle, tt.wantOK)
			}
		})
	}
}

func TestIsStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style int
		want  bool
	}{
		{name: "precision", style: domain.RuneStylePrecision, want: true},
		{name: "domination", style: domain.RuneStyleDomination, want: true},
		{name: "sorcery", style: domain.RuneStyleSorcery, want: true},
		{name: "inspiration", style: domain.RuneStyleInspiration, want: true},
		{name: "resolve", style: domain.RuneStyleResolve, want: true},
		{name: "unknown", style: 9999, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsStyle(tt.style); got != tt.want {
				t.Fatalf("IsStyle(%d) = %v, want %v", tt.style, got, tt.want)
			}
		})
	}
}

func TestRecommendedSecondaryStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		playcounts   []domain.RuneTreePlaycount
		primaryStyle int
		want         int
		wantOK       bool
	}{
		{
			name: "selects highest occurrence",
			playcounts: []domain.RuneTreePlaycount{
				{Tree: domain.RuneStyleDomination, Occurrence: 300},
				{Tree: domain.RuneStyleSorcery, Occurrence: 900},
			},
			primaryStyle: domain.RuneStylePrecision,
			want:         domain.RuneStyleSorcery,
			wantOK:       true,
		},
		{
			name: "ties by lower tree id",
			playcounts: []domain.RuneTreePlaycount{
				{Tree: domain.RuneStyleResolve, Occurrence: 900},
				{Tree: domain.RuneStyleDomination, Occurrence: 900},
			},
			primaryStyle: domain.RuneStylePrecision,
			want:         domain.RuneStyleDomination,
			wantOK:       true,
		},
		{
			name: "ignores primary and invalid styles",
			playcounts: []domain.RuneTreePlaycount{
				{Tree: domain.RuneStylePrecision, Occurrence: 5000},
				{Tree: 9999, Occurrence: 10000},
				{Tree: domain.RuneStyleSorcery, Occurrence: 900},
			},
			primaryStyle: domain.RuneStylePrecision,
			want:         domain.RuneStyleSorcery,
			wantOK:       true,
		},
		{
			name: "returns false without valid candidates",
			playcounts: []domain.RuneTreePlaycount{
				{Tree: domain.RuneStylePrecision, Occurrence: 5000},
				{Tree: 9999, Occurrence: 10000},
			},
			primaryStyle: domain.RuneStylePrecision,
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := RecommendedSecondaryStyle(tt.playcounts, tt.primaryStyle)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("RecommendedSecondaryStyle() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
