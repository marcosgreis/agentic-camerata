package cli

import (
	"context"
	"fmt"
	"time"
)

// LoopFlags provides --loop and --loop-limit flags for session commands.
// Embed this in a command struct to enable recurring execution.
type LoopFlags struct {
	Interval string `name:"loop"       help:"Re-run on a recurring interval (e.g. 5m, 1h)" optional:""`
	Limit    int    `name:"loop-limit" help:"Maximum number of loop iterations (0 = unlimited)" optional:""`
}

// RunWithLoop runs fn once, or repeatedly at the given interval until ctx is cancelled
// or the iteration limit is reached. interval must be a valid time.Duration string (e.g. "5m").
// limit == 0 means unlimited iterations.
func RunWithLoop(ctx context.Context, interval string, limit int, fn func() error) error {
	if interval == "" {
		return fn()
	}
	d, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid loop interval %q: %w", interval, err)
	}
	for i := 0; limit == 0 || i < limit; i++ {
		if err := fn(); err != nil {
			return err
		}
		if limit > 0 && i+1 >= limit {
			break
		}
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}
