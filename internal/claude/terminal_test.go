package claude

import (
	"testing"
)

func TestIsInteractive(t *testing.T) {
	// This test just verifies the function doesn't panic
	// The actual result depends on how tests are run
	result := IsInteractive()
	t.Logf("IsInteractive() = %v", result)
}
