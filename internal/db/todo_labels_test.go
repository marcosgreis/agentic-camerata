package db

import (
	"testing"
)

func TestTodoLabels(t *testing.T) {
	t.Run("add and read labels", func(t *testing.T) {
		db := openTestDB(t)
		todo := &Todo{ID: "lab00001", Status: TodoStatusTodo, Summary: "Labeled"}
		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}

		if err := db.AddTodoLabels(todo.ID, []string{" Work ", "urgent", "work", ""}); err != nil {
			t.Fatalf("AddTodoLabels() error = %v", err)
		}

		labels, err := db.TodoLabels(todo.ID)
		if err != nil {
			t.Fatalf("TodoLabels() error = %v", err)
		}
		if len(labels) != 2 || labels[0] != "urgent" || labels[1] != "Work" {
			t.Errorf("labels = %v, want [urgent Work]", labels)
		}

		// GetTodo loads labels too.
		got, err := db.GetTodo(todo.ID)
		if err != nil {
			t.Fatalf("GetTodo() error = %v", err)
		}
		if got == nil || len(got.Labels) != 2 {
			t.Fatalf("GetTodo() labels = %v, want 2", got.Labels)
		}
	})

	t.Run("create with labels", func(t *testing.T) {
		db := openTestDB(t)
		todo := &Todo{ID: "lab00002", Status: TodoStatusTodo, Summary: "Inline", Labels: []string{"home", "errand"}}
		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}
		labels, err := db.TodoLabels(todo.ID)
		if err != nil {
			t.Fatalf("TodoLabels() error = %v", err)
		}
		if len(labels) != 2 || labels[0] != "errand" || labels[1] != "home" {
			t.Errorf("labels = %v, want [errand home]", labels)
		}
	})

	t.Run("remove labels case-insensitively", func(t *testing.T) {
		db := openTestDB(t)
		todo := &Todo{ID: "lab00003", Status: TodoStatusTodo, Summary: "Remove", Labels: []string{"Work", "urgent"}}
		if err := db.CreateTodo(todo); err != nil {
			t.Fatalf("CreateTodo() error = %v", err)
		}
		if err := db.RemoveTodoLabels(todo.ID, []string{"work", "missing"}); err != nil {
			t.Fatalf("RemoveTodoLabels() error = %v", err)
		}
		labels, err := db.TodoLabels(todo.ID)
		if err != nil {
			t.Fatalf("TodoLabels() error = %v", err)
		}
		if len(labels) != 1 || labels[0] != "urgent" {
			t.Errorf("labels = %v, want [urgent]", labels)
		}
	})

	t.Run("list labels with counts", func(t *testing.T) {
		db := openTestDB(t)
		for i, id := range []string{"lab00004", "lab00005"} {
			todo := &Todo{ID: id, Status: TodoStatusTodo, Summary: "Count", Labels: []string{"shared"}}
			if i == 1 {
				todo.Labels = append(todo.Labels, "solo")
			}
			if err := db.CreateTodo(todo); err != nil {
				t.Fatalf("CreateTodo() error = %v", err)
			}
		}
		counts, err := db.ListTodoLabels()
		if err != nil {
			t.Fatalf("ListTodoLabels() error = %v", err)
		}
		if len(counts) != 2 {
			t.Fatalf("counts = %v, want 2 labels", counts)
		}
		if counts[0].Label != "shared" || counts[0].Count != 2 {
			t.Errorf("counts[0] = %+v, want shared/2", counts[0])
		}
	})

	t.Run("filter by labels uses AND and ignores case", func(t *testing.T) {
		db := openTestDB(t)
		todos := []*Todo{
			{ID: "lab00006", Status: TodoStatusTodo, Summary: "Both", Labels: []string{"work", "urgent"}},
			{ID: "lab00007", Status: TodoStatusTodo, Summary: "One", Labels: []string{"Work"}},
			{ID: "lab00008", Status: TodoStatusTodo, Summary: "None"},
		}
		for _, todo := range todos {
			if err := db.CreateTodo(todo); err != nil {
				t.Fatalf("CreateTodo() error = %v", err)
			}
		}

		one, err := db.SearchTodos(TodoFilter{Labels: []string{"WORK"}})
		if err != nil {
			t.Fatalf("SearchTodos(one label) error = %v", err)
		}
		if len(one) != 2 {
			t.Errorf("one-label results = %d, want 2", len(one))
		}

		both, err := db.SearchTodos(TodoFilter{Labels: []string{"work", "urgent"}})
		if err != nil {
			t.Fatalf("SearchTodos(two labels) error = %v", err)
		}
		if len(both) != 1 || both[0].ID != "lab00006" {
			t.Errorf("two-label results = %v, want lab00006 only", both)
		}

		// ListTodos also carries labels.
		all, err := db.ListTodos("")
		if err != nil {
			t.Fatalf("ListTodos() error = %v", err)
		}
		for _, todo := range all {
			if todo.ID == "lab00006" && len(todo.Labels) != 2 {
				t.Errorf("ListTodos() labels for lab00006 = %v, want 2", todo.Labels)
			}
		}
	})

	t.Run("unknown todo errors", func(t *testing.T) {
		db := openTestDB(t)
		if err := db.AddTodoLabels("nope", []string{"x"}); err == nil {
			t.Error("AddTodoLabels(unknown) = nil, want error")
		}
		if err := db.RemoveTodoLabels("nope", []string{"x"}); err == nil {
			t.Error("RemoveTodoLabels(unknown) = nil, want error")
		}
	})
}
