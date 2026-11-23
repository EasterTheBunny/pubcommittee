package selection_test

import (
	"context"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// fakeStore keeps policy versions in memory and refuses a changed version.
type fakeStore struct {
	policies []data.SelectionPolicy
	logged   data.LogEntry
}

func (f *fakeStore) GetSelectionPolicy(context.Context) (data.SelectionPolicy, error) {
	if len(f.policies) == 0 {
		return data.SelectionPolicy{}, fmt.Errorf("none: %w", data.ErrNotFound)
	}
	return f.policies[len(f.policies)-1], nil
}

func (f *fakeStore) ApplyLogged(_ context.Context, c data.Change, e data.LogEntry) error {
	f.policies = append(f.policies, *c.Policy)
	f.logged = e
	return nil
}
