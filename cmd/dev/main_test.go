package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCommandsKeepDryRunFlagDefaultTrue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  func() *cobra.Command
	}{
		{name: "sync", cmd: syncCmd},
		{name: "watch", cmd: watchCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := tt.cmd()
			flag := cmd.Flags().Lookup("dry-run")
			if flag == nil {
				t.Fatal("dry-run flag is missing")
			}
			if flag.DefValue != "true" {
				t.Fatalf("dry-run default = %q, want %q", flag.DefValue, "true")
			}

			got, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				t.Fatalf("get dry-run flag: %v", err)
			}
			if !got {
				t.Fatal("dry-run flag value = false, want true")
			}
		})
	}
}
