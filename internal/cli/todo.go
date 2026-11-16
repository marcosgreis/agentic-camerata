package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/agentic-camerata/cmt/internal/db"
)

// TodoCmd is the top-level todo command
type TodoCmd struct {
	Add    TodoAddCmd    `cmd:"" help:"Add a new todo"`
	List   TodoListCmd   `cmd:"" help:"List todos"`
	Search TodoSearchCmd `cmd:"" help:"Search todos with filters"`
	Done   TodoDoneCmd   `cmd:"" help:"Mark a todo as done"`
	Undone TodoUndoneCmd `cmd:"" help:"Mark a todo as not done"`
	Update TodoUpdateCmd `cmd:"" help:"Update a todo"`
	Rm     TodoRmCmd     `cmd:"rm" help:"Remove a todo"`
	Label  TodoLabelCmd  `cmd:"label" help:"Add, remove, or list todo labels"`
}

// todoJSON is the JSON representation of a todo with all fields
type todoJSON struct {
	ID             string   `json:"id"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	Status         string   `json:"status"`
	Summary        string   `json:"summary"`
	Date           *string  `json:"date,omitempty"`
	DueDate        *string  `json:"due_date,omitempty"`
	Source         *string  `json:"source,omitempty"`
	URL            *string  `json:"url,omitempty"`
	Channel        *string  `json:"channel,omitempty"`
	Sender         *string  `json:"sender,omitempty"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	FullMessage    *string  `json:"full_message,omitempty"`
	Labels         []string `json:"labels"`
	DeletedAt      *string  `json:"deleted_at,omitempty"`
}

func todoToJSON(t *db.Todo) todoJSON {
	j := todoJSON{
		ID:             t.ID,
		CreatedAt:      t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      t.UpdatedAt.Format(time.RFC3339),
		Status:         string(t.Status),
		Summary:        t.Summary,
		Source:         t.Source,
		URL:            t.URL,
		Channel:        t.Channel,
		Sender:         t.Sender,
		IdempotencyKey: t.IdempotencyKey,
		FullMessage:    t.FullMessage,
		Labels:         t.Labels,
	}
	if j.Labels == nil {
		j.Labels = []string{}
	}
	if t.Date != nil {
		s := t.Date.Format(time.RFC3339)
		j.Date = &s
	}
	if t.DueDate != nil {
		s := t.DueDate.Format(time.RFC3339)
		j.DueDate = &s
	}
	if t.DeletedAt != nil {
		s := t.DeletedAt.Format(time.RFC3339)
		j.DeletedAt = &s
	}
	return j
}

// todayCutoffUTC returns today's local calendar date anchored at UTC midnight.
// Due dates are date-only and stored at UTC midnight, so "overdue" is
// due_date < this cutoff (strictly before today's date).
func todayCutoffUTC() time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// parseDueBefore parses a YYYY-MM-DD --due-before value into a UTC-midnight cutoff.
func parseDueBefore(s string) (*time.Time, error) {
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("invalid due-before %q: expected YYYY-MM-DD format", s)
	}
	return &parsed, nil
}

func printTodosJSON(todos []*db.Todo) error {
	out := make([]todoJSON, len(todos))
	for i, t := range todos {
		out[i] = todoToJSON(t)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// TodoAddCmd adds a new todo
type TodoAddCmd struct {
	Summary     string   `arg:"" help:"Summary of the todo"`
	Source      string   `short:"s" help:"Source (e.g. slack, github, email)" optional:""`
	Channel     string   `short:"c" help:"Channel (e.g. #engineering)" optional:""`
	Sender      string   `short:"f" help:"Sender" optional:""`
	URL         string   `short:"u" help:"URL" optional:""`
	Date        string   `short:"d" help:"Date (YYYY-MM-DD)" optional:""`
	Due         *string  `short:"D" name:"due" help:"Due date (YYYY-MM-DD)" optional:""`
	Key         string   `short:"k" help:"Idempotency key for deduplication" optional:""`
	FullMessage string   `short:"m" name:"full-message" help:"Full message text" optional:""`
	Labels      []string `short:"l" name:"label" help:"Label (repeatable, comma-separated)" sep:"," optional:""`
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
	if c.Due != nil && *c.Due != "" {
		parsed, err := time.Parse("2006-01-02", *c.Due)
		if err != nil {
			return fmt.Errorf("invalid due date %q: expected YYYY-MM-DD format", *c.Due)
		}
		t.DueDate = &parsed
	}
	if c.Key != "" {
		t.IdempotencyKey = &c.Key
	}
	if c.FullMessage != "" {
		t.FullMessage = &c.FullMessage
	}
	t.Labels = c.Labels

	if err := cli.Database().CreateTodo(t); err != nil {
		return fmt.Errorf("create todo: %w", err)
	}

	fmt.Println(t.ID)
	return nil
}

// TodoListCmd lists todos
type TodoListCmd struct {
	Status    string   `short:"s" help:"Filter by status: todo, done, deleted, all" default:"todo"`
	Overdue   bool     `help:"Only overdue todos (due date before today); implies --status todo" optional:""`
	DueBefore string   `name:"due-before" help:"Only todos due before DATE (YYYY-MM-DD)" optional:""`
	SortDue   bool     `name:"sort-due" help:"Sort by due date ascending (nulls last)" optional:""`
	Labels    []string `short:"l" name:"label" help:"Only todos with every given label (repeatable, comma-separated)" sep:"," optional:""`
}

func (c *TodoListCmd) Run(cli *CLI) error {
	filter := db.TodoFilter{SortDue: c.SortDue, Labels: c.Labels}

	switch c.Status {
	case "all", "":
		// no status filter (all non-deleted)
	case "todo":
		s := db.TodoStatusTodo
		filter.Status = &s
	case "done":
		s := db.TodoStatusDone
		filter.Status = &s
	case "deleted":
		filter.OnlyDeleted = true
	default:
		return fmt.Errorf("invalid status %q: must be todo, done, deleted, or all", c.Status)
	}

	// --overdue is pending-only; error if combined with a conflicting --status
	if c.Overdue && c.Status != "todo" {
		statusDesc := c.Status
		if statusDesc == "" {
			statusDesc = "all"
		}
		return fmt.Errorf("--overdue and --status %s are mutually exclusive: --overdue implies --status todo", statusDesc)
	}
	// --overdue and --due-before are mutually exclusive
	if c.Overdue && c.DueBefore != "" {
		return fmt.Errorf("--overdue and --due-before are mutually exclusive")
	}

	if c.Overdue {
		cut := todayCutoffUTC()
		filter.DueBefore = &cut
	} else if c.DueBefore != "" {
		cut, err := parseDueBefore(c.DueBefore)
		if err != nil {
			return err
		}
		filter.DueBefore = cut
	}

	todos, err := cli.Database().SearchTodos(filter)
	if err != nil {
		return fmt.Errorf("list todos: %w", err)
	}

	return printTodosJSON(todos)
}

// TodoSearchCmd searches todos with filters
type TodoSearchCmd struct {
	ID             string   `help:"Filter by ID" optional:""`
	Key            string   `short:"k" help:"Filter by idempotency key" optional:""`
	Status         string   `short:"s" help:"Filter by status: todo, done" optional:""`
	URL            string   `short:"u" help:"Filter by URL" optional:""`
	Sender         string   `short:"f" help:"Filter by sender" optional:""`
	Source         string   `help:"Filter by source" optional:""`
	IncludeDeleted bool     `short:"d" help:"Include soft-deleted todos" optional:""`
	Overdue        bool     `help:"Only overdue todos (due date before today); implies --status todo" optional:""`
	DueBefore      string   `name:"due-before" help:"Only todos due before DATE (YYYY-MM-DD)" optional:""`
	SortDue        bool     `name:"sort-due" help:"Sort by due date ascending (nulls last)" optional:""`
	Labels         []string `short:"l" name:"label" help:"Only todos with every given label (repeatable, comma-separated)" sep:"," optional:""`
}

func (c *TodoSearchCmd) Run(cli *CLI) error {
	filter := db.TodoFilter{
		IncludeDeleted: c.IncludeDeleted,
		Labels:         c.Labels,
	}

	if c.ID != "" {
		filter.ID = &c.ID
	}
	if c.Key != "" {
		filter.IdempotencyKey = &c.Key
	}
	if c.Status != "" {
		switch c.Status {
		case "todo":
			s := db.TodoStatusTodo
			filter.Status = &s
		case "done":
			s := db.TodoStatusDone
			filter.Status = &s
		default:
			return fmt.Errorf("invalid status %q: must be todo or done", c.Status)
		}
	}
	if c.URL != "" {
		filter.URL = &c.URL
	}
	if c.Sender != "" {
		filter.Sender = &c.Sender
	}
	if c.Source != "" {
		filter.Source = &c.Source
	}

	filter.SortDue = c.SortDue

	// --overdue is pending-only; error if combined with --status done
	if c.Overdue && c.Status == "done" {
		return fmt.Errorf("--overdue and --status done are mutually exclusive: --overdue implies --status todo")
	}
	// --overdue and --due-before are mutually exclusive
	if c.Overdue && c.DueBefore != "" {
		return fmt.Errorf("--overdue and --due-before are mutually exclusive")
	}
	// --overdue forces status=todo (pending-only) and applies today cutoff
	if c.Overdue {
		s := db.TodoStatusTodo
		filter.Status = &s
		cut := todayCutoffUTC()
		filter.DueBefore = &cut
	} else if c.DueBefore != "" {
		cut, err := parseDueBefore(c.DueBefore)
		if err != nil {
			return err
		}
		filter.DueBefore = cut
	}

	todos, err := cli.Database().SearchTodos(filter)
	if err != nil {
		return fmt.Errorf("search todos: %w", err)
	}

	return printTodosJSON(todos)
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
	ID          string   `arg:"" help:"Todo ID"`
	Summary     string   `short:"S" help:"New summary" optional:""`
	Source      string   `short:"s" help:"Source" optional:""`
	Channel     string   `short:"c" help:"Channel" optional:""`
	Sender      string   `short:"f" help:"Sender" optional:""`
	URL         string   `short:"u" help:"URL" optional:""`
	Date        string   `short:"d" help:"Date (YYYY-MM-DD)" optional:""`
	Due         *string  `short:"D" name:"due" help:"Due date (YYYY-MM-DD); pass empty to clear" optional:""`
	FullMessage string   `short:"m" name:"full-message" help:"Full message text" optional:""`
	AddLabels   []string `short:"l" name:"label" help:"Label to add (repeatable, comma-separated)" sep:"," optional:""`
	RmLabels    []string `name:"rm-label" help:"Label to remove (repeatable, comma-separated)" sep:"," optional:""`
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
	if c.Due != nil {
		if *c.Due == "" {
			t.DueDate = nil // explicit clear
		} else {
			parsed, err := time.Parse("2006-01-02", *c.Due)
			if err != nil {
				return fmt.Errorf("invalid due date %q: expected YYYY-MM-DD format", *c.Due)
			}
			t.DueDate = &parsed
		}
	}
	if c.FullMessage != "" {
		t.FullMessage = &c.FullMessage
	}

	if err := cli.Database().UpdateTodo(t); err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	if err := cli.Database().AddTodoLabels(t.ID, c.AddLabels); err != nil {
		return fmt.Errorf("add labels: %w", err)
	}
	if err := cli.Database().RemoveTodoLabels(t.ID, c.RmLabels); err != nil {
		return fmt.Errorf("remove labels: %w", err)
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

// printJSON writes v as indented JSON to stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// TodoLabelCmd groups label management commands
type TodoLabelCmd struct {
	Add  TodoLabelAddCmd  `cmd:"" help:"Add labels to a todo"`
	Rm   TodoLabelRmCmd   `cmd:"rm" help:"Remove labels from a todo"`
	List TodoLabelListCmd `cmd:"list" help:"List labels for a todo, or all labels with counts"`
}

// TodoLabelAddCmd adds labels to an existing todo
type TodoLabelAddCmd struct {
	ID     string   `arg:"" help:"Todo ID"`
	Labels []string `arg:"" help:"Labels to add"`
}

func (c *TodoLabelAddCmd) Run(cli *CLI) error {
	if err := cli.Database().AddTodoLabels(c.ID, c.Labels); err != nil {
		return err
	}
	labels, err := cli.Database().TodoLabels(c.ID)
	if err != nil {
		return fmt.Errorf("read labels: %w", err)
	}
	return printJSON(struct {
		ID     string   `json:"id"`
		Labels []string `json:"labels"`
	}{c.ID, labels})
}

// TodoLabelRmCmd removes labels from an existing todo
type TodoLabelRmCmd struct {
	ID     string   `arg:"" help:"Todo ID"`
	Labels []string `arg:"" help:"Labels to remove"`
}

func (c *TodoLabelRmCmd) Run(cli *CLI) error {
	if err := cli.Database().RemoveTodoLabels(c.ID, c.Labels); err != nil {
		return err
	}
	labels, err := cli.Database().TodoLabels(c.ID)
	if err != nil {
		return fmt.Errorf("read labels: %w", err)
	}
	return printJSON(struct {
		ID     string   `json:"id"`
		Labels []string `json:"labels"`
	}{c.ID, labels})
}

// TodoLabelListCmd lists labels for one todo, or every label in use.
type TodoLabelListCmd struct {
	ID string `arg:"" help:"Todo ID (omit to list every label with todo counts)" optional:""`
}

func (c *TodoLabelListCmd) Run(cli *CLI) error {
	if c.ID != "" {
		t, err := cli.Database().GetTodo(c.ID)
		if err != nil {
			return fmt.Errorf("get todo: %w", err)
		}
		if t == nil {
			return fmt.Errorf("todo %q not found", c.ID)
		}
		return printJSON(struct {
			ID     string   `json:"id"`
			Labels []string `json:"labels"`
		}{c.ID, t.Labels})
	}
	labels, err := cli.Database().ListTodoLabels()
	if err != nil {
		return fmt.Errorf("list labels: %w", err)
	}
	return printJSON(struct {
		Labels []db.TodoLabelCount `json:"labels"`
	}{labels})
}
