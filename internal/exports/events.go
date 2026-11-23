package exports

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// day is the day the feed stamps new entries with: the run's UTC day.
func (r *run) day() time.Time { return r.Now().UTC().Truncate(data.Day) }

func (r *run) event(id, kind, title, address string) {
	d := r.day()
	r.events = append(r.events, data.FeedEntry{
		ID: id, Updated: d, Title: title, Kind: kind, Address: address,
		Meta: data.Meta{Version: 1, AsOf: d, Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform},
	})
}

func (r *run) catalogEvents(e data.CatalogEntry) {
	id := e.Bill.ID()
	r.event("catalog:"+id, data.FeedCatalogBill, "Bill "+id+" entered the catalog",
		"/v1/bills?scope=catalog&q="+id)
	if e.Tracked {
		title := "Bill " + id + " is now tracked"
		if len(e.Reasons) > 0 {
			title += " (" + strings.Join(e.Reasons, ", ") + ")"
		}
		r.event("tracked:"+id, data.FeedBillTracked, title, "/v1/bills/"+id)
	}
}

func (r *run) versionEvent(n int, ref data.VersionRef) {
	r.event("version:"+ref.String(), data.FeedVersion,
		"New official version "+ref.Version+" of bill "+ref.Bill.ID(),
		"/v1/bills/"+ref.Bill.ID()+"/v/"+strconv.Itoa(n))
}

// explanationEvent records an explanation revision. Each revision of an
// explanation has its own id, so a correction reaches clients that already
// saw the first text.
func (r *run) explanationEvent(n int, ref data.VersionRef, x data.ChangeExplanation) {
	kind, what := data.FeedExplanation, "New"
	if x.Meta.Version > 1 {
		kind, what = data.FeedExplanationCorrected, "Corrected"
	}
	id := fmt.Sprintf("explanation:%s:%s:%s:r%d", ref, strings.Join(x.Section, "."), x.ProvisionID, x.Meta.Version)
	r.event(id, kind, what+" change explanation for version "+ref.Version+" of bill "+ref.Bill.ID(),
		"/v1/bills/"+ref.Bill.ID()+"/v/"+strconv.Itoa(n)+"/explanations")
}

// policyEvents records every stored selection policy version, oldest first.
func (r *run) policyEvents(ctx context.Context) error {
	latest, err := r.Ports.PublicDocs.GetSelectionPolicy(ctx)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get selection policy: %w", err)
	}
	for v := 1; v <= latest.Version; v++ {
		_, err := r.Ports.PublicDocs.GetSelectionPolicyVersion(ctx, v)
		if errors.Is(err, data.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("get selection policy %d: %w", v, err)
		}
		r.event("policy:"+strconv.Itoa(v), data.FeedPolicy, "Selection policy version "+strconv.Itoa(v),
			"/v1/selection-policy/"+strconv.Itoa(v))
	}
	return nil
}

// logEvents records every administrator log entry, oldest first. The log
// reason is administrator text, so the title never repeats it; the address
// leads to the log.
func (r *run) logEvents(ctx context.Context) error {
	var log []data.LogEntry
	cursor := ""
	for {
		pg, err := r.Ports.PublicDocs.ListLog(ctx, data.Page{Cursor: cursor})
		if err != nil {
			return fmt.Errorf("list log: %w", err)
		}
		log = append(log, pg.Items...)
		if pg.Next == "" {
			break
		}
		cursor = pg.Next
	}
	for _, e := range slices.Backward(log) {
		r.event("log:"+e.ID, data.FeedLogEntry, "Administrator log entry: "+e.Kind, "/v1/moderation-log")
	}
	return nil
}
