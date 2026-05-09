package lcu

import (
	"context"
	"slices"
	"testing"
)

func TestForEachCandidateSkipsDuplicateResolvedEndpoints(t *testing.T) {
	t.Parallel()

	info := connectionInfo{Port: 12345, Password: "secret", Protocol: "http"}
	client := &Client{
		discoverProcessConnections: func(context.Context) []connectionCandidate {
			return []connectionCandidate{
				staticCandidate("process:1", info),
				staticCandidate("process:1:lockfile", info),
			}
		},
	}

	var labels []string
	success, err := client.forEachCandidate(context.Background(), newConnectionAttempt(), func(_ connectionInfo, label string) bool {
		labels = append(labels, label)
		return false
	})
	if err != nil {
		t.Fatalf("forEachCandidate() error = %v", err)
	}
	if success {
		t.Fatal("forEachCandidate() success = true, want false")
	}
	if !slices.Equal(labels, []string{"process:1"}) {
		t.Fatalf("candidate labels = %+v, want [process:1]", labels)
	}
}

func TestForEachCandidateKeepsDistinctCredentials(t *testing.T) {
	t.Parallel()

	client := &Client{
		discoverProcessConnections: func(context.Context) []connectionCandidate {
			return []connectionCandidate{
				staticCandidate("first", connectionInfo{Port: 12345, Password: "first", Protocol: "http"}),
				staticCandidate("second", connectionInfo{Port: 12345, Password: "second", Protocol: "http"}),
			}
		},
	}

	var labels []string
	success, err := client.forEachCandidate(context.Background(), newConnectionAttempt(), func(_ connectionInfo, label string) bool {
		labels = append(labels, label)
		return false
	})
	if err != nil {
		t.Fatalf("forEachCandidate() error = %v", err)
	}
	if success {
		t.Fatal("forEachCandidate() success = true, want false")
	}
	if !slices.Equal(labels, []string{"first", "second"}) {
		t.Fatalf("candidate labels = %+v, want [first second]", labels)
	}
}
