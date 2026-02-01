package claude

import (
	"os"
	"testing"
)

func TestIsInteractive(t *testing.T) {
	// This test just verifies the function doesn't panic
	// The actual result depends on how tests are run
	result := IsInteractive()
	t.Logf("IsInteractive() = %v", result)
}

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

func TestGetSize(t *testing.T) {
	// Skip if not a terminal
	if !isTerminal(os.Stdout.Fd()) {
		t.Skip("stdout is not a terminal")
	}

	width, height, err := getSize(os.Stdout.Fd())
	if err != nil {
		t.Skipf("getSize() failed: %v", err)
	}

	t.Logf("Terminal size: %dx%d", width, height)

	if width <= 0 {
		t.Error("width should be positive")
	}
	if height <= 0 {
		t.Error("height should be positive")
	}
}
