package main

import (
	"context"

	"github.com/alan/cherry-picker/cmd"
	"github.com/alan/cherry-picker/internal/commands"
	"github.com/alan/cherry-picker/internal/depmerger"
	"github.com/alan/cherry-picker/internal/github"
	"github.com/alan/cherry-picker/internal/state"
)

// defaultConfigFile is the unified tool's default config+state path.
const defaultConfigFile = "cherry-picker.yaml"

// The load/save adapters bridge the single unified state file to the
// per-subsystem in-memory types the existing commands operate on. Loads project
// a view; saves reconcile a (possibly mutated) view back through state.Update, so
// a concurrent daemon writer is never clobbered (see internal/state merge).

func loadCherry(f string) (*cmd.Config, error) {
	c, err := state.Load(f)
	if err != nil {
		return nil, err
	}
	return c.CherryView(), nil
}

func saveCherry(f string, v *cmd.Config) error {
	return state.Update(f, func(cur *state.Config) error {
		cur.MergeCherryView(v)
		return nil
	})
}

func saveDep(f string, v *depmerger.Config) error {
	return state.Update(f, func(cur *state.Config) error {
		cur.MergeDepView(v)
		return nil
	})
}

// loadStateAndClient loads the unified state and builds a GitHub client for the
// configured repository.
func loadStateAndClient(ctx context.Context, configFile string) (*github.Client, *state.Config, error) {
	st, err := state.Load(configFile)
	if err != nil {
		return nil, nil, err
	}
	client, _, err := commands.InitializeGitHubClient(ctx, st.CherryView())
	if err != nil {
		return nil, nil, err
	}
	return client, st, nil
}

// prTrackedInCherry reports whether prNumber is tracked in the cherry-pick section.
func prTrackedInCherry(st *state.Config, prNumber int) bool {
	for i := range st.CherryPicks.TrackedPRs {
		if st.CherryPicks.TrackedPRs[i].Number == prNumber {
			return true
		}
	}
	return false
}
