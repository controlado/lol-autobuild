package runes

import (
	"testing"

	"github.com/controlado/lol-autobuild/internal/ports"
)

func TestStyleForKeystone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		keystone  int
		wantStyle int
		wantOK    bool
	}{
		{name: "precision", keystone: ports.RuneKeystonePressTheAttack, wantStyle: ports.RuneStylePrecision, wantOK: true},
		{name: "domination", keystone: ports.RuneKeystoneElectrocute, wantStyle: ports.RuneStyleDomination, wantOK: true},
		{name: "sorcery", keystone: ports.RuneKeystoneArcaneComet, wantStyle: ports.RuneStyleSorcery, wantOK: true},
		{name: "resolve", keystone: ports.RuneKeystoneGraspOfTheUndying, wantStyle: ports.RuneStyleResolve, wantOK: true},
		{name: "inspiration", keystone: ports.RuneKeystoneFirstStrike, wantStyle: ports.RuneStyleInspiration, wantOK: true},
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
		{name: "precision", style: ports.RuneStylePrecision, want: true},
		{name: "domination", style: ports.RuneStyleDomination, want: true},
		{name: "sorcery", style: ports.RuneStyleSorcery, want: true},
		{name: "inspiration", style: ports.RuneStyleInspiration, want: true},
		{name: "resolve", style: ports.RuneStyleResolve, want: true},
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
		playcounts   []ports.RuneTreePlaycount
		primaryStyle int
		want         int
		wantOK       bool
	}{
		{
			name: "selects highest occurrence",
			playcounts: []ports.RuneTreePlaycount{
				{Tree: ports.RuneStyleDomination, Occurrence: 300},
				{Tree: ports.RuneStyleSorcery, Occurrence: 900},
			},
			primaryStyle: ports.RuneStylePrecision,
			want:         ports.RuneStyleSorcery,
			wantOK:       true,
		},
		{
			name: "ties by lower tree id",
			playcounts: []ports.RuneTreePlaycount{
				{Tree: ports.RuneStyleResolve, Occurrence: 900},
				{Tree: ports.RuneStyleDomination, Occurrence: 900},
			},
			primaryStyle: ports.RuneStylePrecision,
			want:         ports.RuneStyleDomination,
			wantOK:       true,
		},
		{
			name: "ignores primary and invalid styles",
			playcounts: []ports.RuneTreePlaycount{
				{Tree: ports.RuneStylePrecision, Occurrence: 5000},
				{Tree: 9999, Occurrence: 10000},
				{Tree: ports.RuneStyleSorcery, Occurrence: 900},
			},
			primaryStyle: ports.RuneStylePrecision,
			want:         ports.RuneStyleSorcery,
			wantOK:       true,
		},
		{
			name: "returns false without valid candidates",
			playcounts: []ports.RuneTreePlaycount{
				{Tree: ports.RuneStylePrecision, Occurrence: 5000},
				{Tree: 9999, Occurrence: 10000},
			},
			primaryStyle: ports.RuneStylePrecision,
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
