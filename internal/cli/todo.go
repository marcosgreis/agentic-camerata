package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/db"
)

// TodoCmd is the top-level todo command
type TodoCmd struct {
	Add    TodoAddCmd    `cmd:"" help:"Add a new todo"`
	List   TodoListCmd   `cmd:"" help:"List todos"`
	Done   TodoDoneCmd   `cmd:"" help:"Mark a todo as done"`
	Undone TodoUndoneCmd `cmd:"" help:"Mark a todo as not done"`
	Update TodoUpdateCmd `cmd:"" help:"Update a todo"`
	Rm     TodoRmCmd     `cmd:"rm" help:"Remove a todo"`
}

// TodoAddCmd adds a new todo
type TodoAddCmd struct {
	Summary string `arg:"" help:"Summary of the todo"`
	Source  string `short:"s" help:"Source (e.g. slack, github, email)" optional:""`
	Channel string `short:"c" help:"Channel (e.g. #engineering)" optional:""`
	Sender  string `short:"f" help:"Sender" optional:""`
	URL     string `short:"u" help:"URL" optional:""`
	Date    string `short:"d" help:"Date (YYYY-MM-DD)" optional:""`
	Key         string `short:"k" help:"Idempotency key for deduplication" optional:""`
	FullMessage string `short:"m" name:"full-message" help:"Full message text" optional:""`
}

func (c *TodoAddCmd) Run(cli *CLI) error {
	t := &db.Todo{
		ID:      uuid.New().String()[:8],
		Status:  db.TodoStatusTodo,
		Summary: c.Summary,
	}

	if c.Source != "" {
		t.Source = &c.Source
	}
	if c.Channel != "" {
		t.Channel = &c.Channel
	}
	if c.Sender != "" {
		t.Sender = &c.Sender
	}
	if c.URL != "" {
		t.URL = &c.URL
	}
	if c.Date != "" {
		parsed, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			return fmt.Errorf("invalid date %q: expected YYYY-MM-DD format", c.Date)
		}
		t.Date = &parsed
	}
	if c.Key != "" {
		t.IdempotencyKey = &c.Key
	}
	if c.FullMessage != "" {
		t.FullMessage = &c.FullMessage
	}

	if err := cli.Database().CreateTodo(t); err != nil {
		return fmt.Errorf("create todo: %w", err)
	}

	fmt.Println(t.ID)
	return nil
}

// TodoListCmd lists todos
type TodoListCmd struct {
	Status string `short:"s" help:"Filter by status: todo, done, deleted, all" default:"todo"`
}

func (c *TodoListCmd) Run(cli *CLI) error {
	var status db.TodoStatus
	switch c.Status {
	case "all", "":
		status = ""
	case "todo":
		status = db.TodoStatusTodo
	case "done":
		status = db.TodoStatusDone
	case "deleted":
		status = db.TodoStatusDeleted
	default:
		return fmt.Errorf("invalid status %q: must be todo, done, deleted, or all", c.Status)
	}

	todos, err := cli.Database().ListTodos(status)
	if err != nil {
		return fmt.Errorf("list todos: %w", err)
	}

	if len(todos) == 0 {
		fmt.Println("No todos found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tDATE\tSOURCE\tSUMMARY")

	for _, t := range todos {
		dateStr := "—"
		if t.Date != nil {
			dateStr = t.Date.Format("2006-01-02")
		}
		sourceStr := "—"
		if t.Source != nil {
			sourceStr = *t.Source
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID, t.Status, dateStr, sourceStr, t.Summary)
	}

	return w.Flush()
}

// TodoDoneCmd marks a todo as done
type TodoDoneCmd struct {
	ID string `arg:"" help:"Todo ID"`
}

func (c *TodoDoneCmd) Run(cli *CLI) error {
	t, err := cli.Database().GetTodo(c.ID)
	if err != nil {
		return fmt.Errorf("get todo: %w", err)
	}
	if t == nil {
		return fmt.Errorf("todo %q not found", c.ID)
	}

	t.Status = db.TodoStatusDone
	if err := cli.Database().UpdateTodo(t); err != nil {
		return fmt.Errorf("update todo: %w", err)
	}

	fmt.Printf("Todo %s marked as done.\n", t.ID)
	return nil
}

// TodoUndoneCmd marks a todo as not done
type TodoUndoneCmd struct {
	ID string `arg:"" help:"Todo ID"`
}

func (c *TodoUndoneCmd) Run(cli *CLI) error {
	t, err := cli.Database().GetTodo(c.ID)
	if err != nil {
		return fmt.Errorf("get todo: %w", err)
	}
	if t == nil {
		return fmt.Errorf("todo %q not found", c.ID)
	}

	t.Status = db.TodoStatusTodo
	if err := cli.Database().UpdateTodo(t); err != nil {
		return fmt.Errorf("update todo: %w", err)
	}

	fmt.Printf("Todo %s marked as todo.\n", t.ID)
	return nil
}

// TodoUpdateCmd updates fields of a todo
type TodoUpdateCmd struct {
	ID      string `arg:"" help:"Todo ID"`
	Summary string `short:"S" help:"New summary" optional:""`
	Source  string `short:"s" help:"Source" optional:""`
	Channel string `short:"c" help:"Channel" optional:""`
	Sender  string `short:"f" help:"Sender" optional:""`
	URL     string `short:"u" help:"URL" optional:""`
	Date        string `short:"d" help:"Date (YYYY-MM-DD)" optional:""`
	FullMessage string `short:"m" name:"full-message" help:"Full message text" optional:""`
}

func (c *TodoUpdateCmd) Run(cli *CLI) error {
	t, err := cli.Database().GetTodo(c.ID)
	if err != nil {
		return fmt.Errorf("get todo: %w", err)
	}
	if t == nil {
		return fmt.Errorf("todo %q not found", c.ID)
	}

	if c.Summary != "" {
		t.Summary = c.Summary
	}
	if c.Source != "" {
		t.Source = &c.Source
	}
	if c.Channel != "" {
		t.Channel = &c.Channel
	}
	if c.Sender != "" {
		t.Sender = &c.Sender
	}
	if c.URL != "" {
		t.URL = &c.URL
	}
	if c.Date != "" {
		parsed, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			return fmt.Errorf("invalid date %q: expected YYYY-MM-DD format", c.Date)
		}
		t.Date = &parsed
	}
	if c.FullMessage != "" {
		t.FullMessage = &c.FullMessage
	}

	if err := cli.Database().UpdateTodo(t); err != nil {
		return fmt.Errorf("update todo: %w", err)
	}

	fmt.Printf("Todo %s updated.\n", t.ID)
	return nil
}

// TodoRmCmd removes a todo
type TodoRmCmd struct {
	ID string `arg:"" help:"Todo ID"`
}

func (c *TodoRmCmd) Run(cli *CLI) error {
	if err := cli.Database().DeleteTodo(c.ID); err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}

	fmt.Printf("Todo %s removed.\n", c.ID)
	return nil
}
