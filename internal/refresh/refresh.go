// Package refresh orchestrates a full scrape of both subsystems (cherry-picks
// and dependencies) into a single unified state.Config, using one shared
// GitHub client. It is the single fetch code path shared by the unified `fetch`
// command and the daemon tick.
package refresh

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alan/cherry-picker/cmd/fetch"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/alan/cherry-picker/internal/state"
)

// All scrapes both subsystems into c in place. It attempts both even if one
// fails (so a transient error in one subsystem does not starve the other),
// applies whatever each produced, sets LastFetchDate last, and returns the
// joined errors. Callers persist the result via state.Update.
func All(ctx context.Context, client *github.Client, c *state.Config) error {
	var errs []error

	// Cherry-picks. Compute the search window from LastFetchDate before it is
	// overwritten below.
	cv := c.CherryView()
	if since, err := fetch.SinceForFetch(cv); err != nil {
		errs = append(errs, fmt.Errorf("cherry-pick since date: %w", err))
	} else if err := fetch.RefreshCherry(ctx, client, cv, since); err != nil {
		errs = append(errs, fmt.Errorf("cherry-pick refresh: %w", err))
	}
	c.ApplyCherryView(cv)

	// Dependencies.
	dv := c.DepView()
	if err := depmerger.RefreshDeps(ctx, client, dv); err != nil {
		errs = append(errs, fmt.Errorf("dependency refresh: %w", err))
	}
	c.ApplyDepView(dv)

	now := time.Now()
	c.LastFetchDate = &now

	return errors.Join(errs...)
}
