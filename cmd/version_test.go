package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/version"
)

func TestRunVersionUsesResolvedVersion(t *testing.T) {
	original := version.Version
	t.Cleanup(func() {
		version.Version = original
	})
	version.Version = "2.3.4"

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	runVersion(cmd, nil)

	if got, want := output.String(), "v2.3.4\n"; got != want {
		t.Fatalf("runVersion() output = %q, want %q", got, want)
	}
}
