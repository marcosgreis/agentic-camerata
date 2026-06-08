package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirDefault(t *testing.T) {
	t.Setenv(EnvDir, "")
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".agentic-camerata", "catalog")
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("Dir() = %q, want absolute path", dir)
	}
}

func TestDirEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(EnvDir, tmp)
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != tmp {
		t.Errorf("Dir() = %q, want %q", dir, tmp)
	}
}

func TestDirTildeExpansion(t *testing.T) {
	t.Setenv(EnvDir, "~/some/catalog")
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "some", "catalog")
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}

func writeTempMD(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "source.md")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

func TestSaveBasename(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "hello")

	dest, err := Save(src, "", false)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Base(dest) != "source.md" {
		t.Errorf("dest basename = %q, want source.md", filepath.Base(dest))
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("dest content = %q, want hello", string(data))
	}
}

func TestSaveCustomNameAddsMD(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "x")

	dest, err := Save(src, "mynote", false)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Base(dest) != "mynote.md" {
		t.Errorf("dest basename = %q, want mynote.md", filepath.Base(dest))
	}
}

func TestSaveCustomNameWithMD(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "x")

	dest, err := Save(src, "mynote.md", false)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Base(dest) != "mynote.md" {
		t.Errorf("dest basename = %q, want mynote.md", filepath.Base(dest))
	}
}

func TestSaveConflictAndForce(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "first")

	if _, err := Save(src, "note", false); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if _, err := Save(src, "note", false); err == nil {
		t.Errorf("expected conflict error on second Save without force")
	}
	if _, err := Save(src, "note", true); err != nil {
		t.Errorf("Save with force: %v", err)
	}
}

func TestSaveRejectsNonMD(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	dir := t.TempDir()
	p := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Save(p, "", false); err == nil {
		t.Errorf("expected error for non-.md file")
	}
}

func TestListEmptyWhenMissing(t *testing.T) {
	t.Setenv(EnvDir, filepath.Join(t.TempDir(), "does-not-exist"))
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %d entries, want 0", len(entries))
	}
}

func TestListMetadata(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "content here")
	if _, err := Save(src, "note", false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List() = %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Name != "note.md" {
		t.Errorf("Name = %q, want note.md", e.Name)
	}
	if e.Size != int64(len("content here")) {
		t.Errorf("Size = %d, want %d", e.Size, len("content here"))
	}
	if e.ModTime == 0 {
		t.Errorf("ModTime = 0, want non-zero")
	}
}

func TestPathFoundAndMissing(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "x")
	if _, err := Save(src, "note", false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := Path("note.md"); err != nil {
		t.Errorf("Path(note.md): %v", err)
	}
	if _, err := Path("note"); err != nil {
		t.Errorf("Path(note) without suffix: %v", err)
	}
	if _, err := Path("nope.md"); err == nil {
		t.Errorf("expected error for missing entry")
	}
}

func TestRemove(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	src := writeTempMD(t, "x")
	if _, err := Save(src, "note", false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := Remove("note.md"); err != nil {
		t.Errorf("Remove: %v", err)
	}
	if _, err := Path("note.md"); err == nil {
		t.Errorf("entry should be gone after Remove")
	}
	if err := Remove("note.md"); err == nil {
		t.Errorf("expected error removing missing entry")
	}
}
