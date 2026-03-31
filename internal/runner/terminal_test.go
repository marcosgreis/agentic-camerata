package runner

import (
	"os"
	"testing"
)

func TestIsTerminal(t *testing.T) {
	// Test with stdin
	result := isTerminal(os.Stdin.Fd())
	t.Logf("isTerminal(stdin) = %v", result)

	// Test with stdout
	result = isTerminal(os.Stdout.Fd())
	t.Logf("isTerminal(stdout) = %v", result)
}

func TestMakeRawAndRestore(t *testing.T) {
	// Skip if not a terminal (common in CI)
	if !isTerminal(os.Stdin.Fd()) {
		t.Skip("stdin is not a terminal")
	}

	// This is a basic test to ensure makeRaw/restoreTerminal don't panic
	state, err := makeRaw(os.Stdin.Fd())
	if err != nil {
		// This may fail in non-interactive environments
		t.Skipf("makeRaw() failed (expected in non-TTY): %v", err)
	}

	// Restore immediately
	restoreTerminal(os.Stdin.Fd(), state)
}

func TestRestoreTerminalWithNilState(t *testing.T) {
	// Should not panic
	restoreTerminal(os.Stdin.Fd(), nil)
}
