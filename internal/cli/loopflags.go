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
//
// fn receives a pointer to an interrupted flag. When looping, callers should pass this as
// RunOptions.Interrupted so the runner can signal a user-initiated exit (Ctrl+C). If the
// flag is set after fn returns, the loop stops instead of waiting for the next interval.
func RunWithLoop(ctx context.Context, interval string, limit int, fn func(interrupted *bool) error) error {
	if interval == "" {
		var notUsed bool
		return fn(&notUsed)
	}
	d, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid loop interval %q: %w", interval, err)
	}
	var interrupted bool
	for i := 0; limit == 0 || i < limit; i++ {
		interrupted = false
		if err := fn(&interrupted); err != nil {
			return err
		}
		if interrupted {
			return nil
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
