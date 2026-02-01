package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileFlags_ResolveFiles_EmptySlice(t *testing.T) {
	ff := FileFlags{}
	result, err := ff.ResolveFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestFileFlags_ResolveFiles_DirectFiles(t *testing.T) {
	// Create temporary files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.md")
	file2 := filepath.Join(tmpDir, "file2.md")
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ff := FileFlags{Files: []string{file1, file2}}
	result, err := ff.ResolveFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}
	if result[0] != file1 {
		t.Errorf("expected %s, got %s", file1, result[0])
	}
	if result[1] != file2 {
		t.Errorf("expected %s, got %s", file2, result[1])
	}
}

func TestFileFlags_ResolveFiles_NonExistentFile(t *testing.T) {
	ff := FileFlags{Files: []string{"/nonexistent/path/to/file.md"}}
	_, err := ff.ResolveFiles()
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	if err.Error() != "file not found: /nonexistent/path/to/file.md" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPrependFilesToTask_EmptyFiles(t *testing.T) {
	result := PrependFilesToTask([]string{}, "my task")
	if result != "my task" {
		t.Errorf("expected 'my task', got '%s'", result)
	}
}

func TestPrependFilesToTask_SingleFile(t *testing.T) {
	result := PrependFilesToTask([]string{"file1.md"}, "my task")
	if result != "file1.md my task" {
		t.Errorf("expected 'file1.md my task', got '%s'", result)
	}
}

func TestPrependFilesToTask_MultipleFiles(t *testing.T) {
	result := PrependFilesToTask([]string{"file1.md", "file2.md"}, "my task")
	if result != "file1.md file2.md my task" {
		t.Errorf("expected 'file1.md file2.md my task', got '%s'", result)
	}
}

func TestPrependFilesToTask_EmptyTask(t *testing.T) {
	result := PrependFilesToTask([]string{"file1.md"}, "")
	if result != "file1.md " {
		t.Errorf("expected 'file1.md ', got '%s'", result)
	}
}
