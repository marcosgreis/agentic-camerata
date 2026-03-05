package db

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func strp(s string) *string { return &s }
func timep(t time.Time) *time.Time { return &t }

func TestTodoCRUD(t *testing.T) {
	t.Run("create and get todo", func(t *testing.T) {
		db := openTestDB(t)

		todo := &Todo{
			ID:      "abc12345",
			Status:  TodoStatusTodo,
			Summary: "Test summary",
			Source:  strp("github"),
			URL:     strp("https://example.com"),
			Channel: strp("#eng"),
			Sender:  strp("alice"),
			Date:    timep(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
		}

		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}

		got, err := db.GetTodo(todo.ID)
		if err != nil {
			t.Fatalf("GetTodo() error = %v", err)
		}
		if got == nil {
			t.Fatal("GetTodo() returned nil")
		}
		if got.ID != todo.ID {
			t.Errorf("ID = %q, want %q", got.ID, todo.ID)
		}
		if got.Status != TodoStatusTodo {
			t.Errorf("Status = %q, want %q", got.Status, TodoStatusTodo)
		}
		if got.Summary != "Test summary" {
			t.Errorf("Summary = %q, want %q", got.Summary, "Test summary")
		}
		if got.Source == nil || *got.Source != "github" {
			t.Errorf("Source = %v, want %q", got.Source, "github")
		}
		if got.URL == nil || *got.URL != "https://example.com" {
			t.Errorf("URL = %v, want %q", got.URL, "https://example.com")
		}
		if got.Channel == nil || *got.Channel != "#eng" {
			t.Errorf("Channel = %v, want %q", got.Channel, "#eng")
		}
		if got.Sender == nil || *got.Sender != "alice" {
			t.Errorf("Sender = %v, want %q", got.Sender, "alice")
		}
		if got.Date == nil {
			t.Error("Date is nil, want non-nil")
		}
	})

	t.Run("get non-existent todo returns nil", func(t *testing.T) {
		db := openTestDB(t)
		got, err := db.GetTodo("nonexist")
		if err != nil {
			t.Fatalf("GetTodo() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetTodo() = %v, want nil", got)
		}
	})

	t.Run("create todo with nullable fields nil", func(t *testing.T) {
		db := openTestDB(t)
		todo := &Todo{
			ID:      "null1234",
			Status:  TodoStatusTodo,
			Summary: "No optional fields",
		}
		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}
		got, err := db.GetTodo(todo.ID)
		if err != nil {
			t.Fatalf("GetTodo() error = %v", err)
		}
		if got.Source != nil || got.URL != nil || got.Channel != nil || got.Sender != nil || got.Date != nil {
			t.Error("expected all nullable fields to be nil")
		}
	})

	t.Run("update todo", func(t *testing.T) {
		db := openTestDB(t)
		todo := &Todo{
			ID:      "upd12345",
			Status:  TodoStatusTodo,
			Summary: "Original",
		}
		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}

		todo.Status = TodoStatusDone
		todo.Summary = "Updated"
		todo.Source = strp("slack")
		if err := db.UpdateTodo(todo); err != nil {
			t.Fatalf("UpdateTodo() error = %v", err)
		}

		got, err := db.GetTodo(todo.ID)
		if err != nil {
			t.Fatalf("GetTodo() error = %v", err)
		}
		if got.Status != TodoStatusDone {
			t.Errorf("Status = %q, want %q", got.Status, TodoStatusDone)
		}
		if got.Summary != "Updated" {
			t.Errorf("Summary = %q, want %q", got.Summary, "Updated")
		}
		if got.Source == nil || *got.Source != "slack" {
			t.Errorf("Source = %v, want %q", got.Source, "slack")
		}
	})

	t.Run("delete todo", func(t *testing.T) {
		db := openTestDB(t)
		todo := &Todo{
			ID:      "del12345",
			Status:  TodoStatusTodo,
			Summary: "To delete",
		}
		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}
		if err := db.DeleteTodo(todo.ID); err != nil {
			t.Fatalf("DeleteTodo() error = %v", err)
		}
		got, err := db.GetTodo(todo.ID)
		if err != nil {
			t.Fatalf("GetTodo() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetTodo() after delete = %v, want nil", got)
		}
	})
}

func TestIdempotencyKey(t *testing.T) {
	t.Run("same key returns same ID, no duplicate row", func(t *testing.T) {
		db := openTestDB(t)
		key := "my-key"
		t1 := &Todo{ID: "idem0001", Status: TodoStatusTodo, Summary: "First", IdempotencyKey: &key}
		if err := db.CreateTodo(t1); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}

		t2 := &Todo{ID: "idem0002", Status: TodoStatusTodo, Summary: "Second", IdempotencyKey: &key}
		if err := db.CreateTodo(t2); err != nil {
			t.Fatalf("CreateTodo() second call error = %v", err)
		}
		if t2.ID != t1.ID {
			t.Errorf("second call ID = %q, want %q", t2.ID, t1.ID)
		}

		all, err := db.ListTodos("")
		if err != nil {
			t.Fatalf("ListTodos() error = %v", err)
		}
		if len(all) != 1 {
			t.Errorf("len(all) = %d, want 1 (no duplicate)", len(all))
		}
	})

	t.Run("different keys create distinct todos", func(t *testing.T) {
		db := openTestDB(t)
		k1, k2 := "key-a", "key-b"
		t1 := &Todo{ID: "diff0001", Status: TodoStatusTodo, Summary: "A", IdempotencyKey: &k1}
		t2 := &Todo{ID: "diff0002", Status: TodoStatusTodo, Summary: "B", IdempotencyKey: &k2}
		if err := db.CreateTodo(t1); err != nil {
			t.Fatalf("CreateTodo() t1 error = %v", err)
		}
		if err := db.CreateTodo(t2); err != nil {
			t.Fatalf("CreateTodo() t2 error = %v", err)
		}

		all, err := db.ListTodos("")
		if err != nil {
			t.Fatalf("ListTodos() error = %v", err)
		}
		if len(all) != 2 {
			t.Errorf("len(all) = %d, want 2", len(all))
		}
	})

	t.Run("no key allows duplicate todos", func(t *testing.T) {
		db := openTestDB(t)
		t1 := &Todo{ID: "nokey001", Status: TodoStatusTodo, Summary: "No key"}
		t2 := &Todo{ID: "nokey002", Status: TodoStatusTodo, Summary: "No key"}
		if err := db.CreateTodo(t1); err != nil {
			t.Fatalf("CreateTodo() t1 error = %v", err)
		}
		if err := db.CreateTodo(t2); err != nil {
			t.Fatalf("CreateTodo() t2 error = %v", err)
		}

		all, err := db.ListTodos("")
		if err != nil {
			t.Fatalf("ListTodos() error = %v", err)
		}
		if len(all) != 2 {
			t.Errorf("len(all) = %d, want 2", len(all))
		}
	})
}

func TestListTodos(t *testing.T) {
	t.Run("list all todos", func(t *testing.T) {
		db := openTestDB(t)
		todos := []*Todo{
			{ID: "list0001", Status: TodoStatusTodo, Summary: "Item 1"},
			{ID: "list0002", Status: TodoStatusDone, Summary: "Item 2"},
			{ID: "list0003", Status: TodoStatusTodo, Summary: "Item 3"},
		}
		for _, t2 := range todos {
			if err := db.CreateTodo(t2); err != nil {
				t.Fatalf("CreateTodo() error = %v", err)
			}
		}

		all, err := db.ListTodos("")
		if err != nil {
			t.Fatalf("ListTodos() error = %v", err)
		}
		if len(all) != 3 {
			t.Errorf("len(all) = %d, want 3", len(all))
		}
	})

	t.Run("filter by todo status", func(t *testing.T) {
		db := openTestDB(t)
		todos := []*Todo{
			{ID: "filt0001", Status: TodoStatusTodo, Summary: "Pending"},
			{ID: "filt0002", Status: TodoStatusDone, Summary: "Done"},
		}
		for _, t2 := range todos {
			if err := db.CreateTodo(t2); err != nil {
				t.Fatalf("CreateTodo() error = %v", err)
			}
		}

		pending, err := db.ListTodos(TodoStatusTodo)
		if err != nil {
			t.Fatalf("ListTodos(todo) error = %v", err)
		}
		if len(pending) != 1 || pending[0].Summary != "Pending" {
			t.Errorf("ListTodos(todo) = %v, want 1 pending item", pending)
		}

		done, err := db.ListTodos(TodoStatusDone)
		if err != nil {
			t.Fatalf("ListTodos(done) error = %v", err)
		}
		if len(done) != 1 || done[0].Summary != "Done" {
			t.Errorf("ListTodos(done) = %v, want 1 done item", done)
		}
	})
}
