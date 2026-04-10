package token

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRefreshCmdRequiredFlags(t *testing.T) {
	cmd := NewRefreshCmd()

	tokenIDFlag := cmd.Flag("token_id")
	if tokenIDFlag == nil {
		t.Fatalf("token_id flag was not registered")
	}

	if !isRequiredFlag(tokenIDFlag.Annotations) {
		t.Fatalf("token_id must remain a required flag")
	}

	minimumDurationFlag := cmd.Flag("minimum_duration")
	if minimumDurationFlag == nil {
		t.Fatalf("minimum_duration flag was not registered")
	}

	if isRequiredFlag(minimumDurationFlag.Annotations) {
		t.Fatalf("minimum_duration must not be required when default is zero")
	}
}

func isRequiredFlag(annotations map[string][]string) bool {
	values, ok := annotations[cobra.BashCompOneRequiredFlag]
	if !ok || len(values) == 0 {
		return false
	}

	return values[0] == "true"
}
