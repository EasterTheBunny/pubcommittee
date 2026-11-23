package selection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const hoursPerDay = 24

// PolicyStore is what Publish needs: the stored policy versions and the
// logged policy write.
type PolicyStore interface {
	GetSelectionPolicy(ctx context.Context) (data.SelectionPolicy, error)
	ApplyLogged(ctx context.Context, c data.Change, e data.LogEntry) error
}

// Publish validates a policy version and stores it, immutable, together with
// its public log entry (reason required), atomically. It is the operation the
// administrator action (M1-12) calls; nothing here decides whether to publish.
// A version below the latest is refused. Republishing identical content is a
// no-op and different content for a stored version is data.ErrConflict, both
// by the store. Tracked bills are not touched: policy changes apply going forward.
func Publish(ctx context.Context, s PolicyStore, p data.SelectionPolicy, reason string, at time.Time) error {
	return PublishAs(ctx, s, p, reason, at, nil)
}

// PublishAs is Publish for an administrator action: the public log entry
// names actor (operator id and public name). A nil actor logs none.
func PublishAs(
	ctx context.Context, s PolicyStore, p data.SelectionPolicy, reason string, at time.Time, actor *data.LogOperator,
) error {
	if reason == "" {
		return fmt.Errorf("selection policy %d: a reason is required: %w", p.Version, data.ErrInvalid)
	}
	if err := Validate(p); err != nil {
		return err
	}
	latest, err := s.GetSelectionPolicy(ctx)
	switch {
	case err == nil:
		if p.Version < latest.Version {
			return fmt.Errorf("selection policy %d is below the latest, %d: %w", p.Version, latest.Version, data.ErrInvalid)
		}
		if p.Version == latest.Version {
			return republish(latest, p)
		}
	case errors.Is(err, data.ErrNotFound):
	default:
		return fmt.Errorf("read latest selection policy: %w", err)
	}
	p.Rules = Rules(p)
	meta := data.Meta{Version: 1, AsOf: at.UTC(), Provenance: data.ProvenanceSource, ContentClass: data.ContentPlatform}
	p.Meta = meta
	entry := data.LogEntry{
		ID: fmt.Sprintf("selection-policy-%d", p.Version), At: at.UTC().Truncate(hoursPerDay * time.Hour),
		Kind: "selection-policy", Subject: fmt.Sprintf("selection-policy/%d", p.Version), Reason: reason,
		Actor: actor, Meta: meta,
	}
	if err := s.ApplyLogged(ctx, data.Change{Policy: &p}, entry); err != nil {
		return fmt.Errorf("publish selection policy %d: %w", p.Version, err)
	}
	return nil
}

// republish accepts a second publication of the stored latest version when its
// content is the same apart from Meta, and refuses it otherwise.
func republish(stored, p data.SelectionPolicy) error {
	p.Rules = Rules(p)
	stored.Meta, p.Meta = data.Meta{}, data.Meta{}
	a, errA := json.Marshal(stored)
	b, errB := json.Marshal(p)
	if errA != nil || errB != nil || string(a) != string(b) {
		return fmt.Errorf("selection policy %d is published with different content; publish a new version: %w",
			p.Version, data.ErrConflict)
	}
	return nil
}
