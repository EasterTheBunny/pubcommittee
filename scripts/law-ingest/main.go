// Command law-ingest runs the two M1-07 steps over a local object store, as
// separate commands: nothing chains them (design/tasks § Example data and
// local triggers). The job-ingest command will call the same step functions.
//
//	law-ingest law  -store DIR [-titles 3,42] [-base URL]    store the newest release point
//	law-ingest cite -store DIR -bills fixtures/bills BILL... resolve citations of fixture bills
//
// "law" fetches from the Law Revision Counsel (politeness rules apply); a
// run with no new release point and no -titles fetches nothing and changes
// nothing. "cite" first stores the fixture bills' versions, then runs the
// citation step against the current release point.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/adapters/uscode"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/lawingest"
)

// billIDParts is the number of dash-separated parts of a bill id such as us-119-hr-4313.
const billIDParts = 4

var errUsage = errors.New("usage error")

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "law-ingest:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("want a command, law or cite: %w", errUsage)
	}
	fl := flag.NewFlagSet(args[0], flag.ContinueOnError)
	storeDir := fl.String("store", "", "object store directory")
	titles := fl.String("titles", "", "law: comma-separated title numbers, or \"all\"")
	base := fl.String("base", "", "law: base URL of the law source (default the Law Revision Counsel)")
	bills := fl.String("bills", "fixtures/bills", "cite: fixture bills directory")
	if err := fl.Parse(args[1:]); err != nil {
		return fmt.Errorf("flags: %w", err)
	}
	store, err := fsobject.New(fsobject.Config{Root: *storeDir})
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	ctx := context.Background()
	switch args[0] {
	case "law":
		return runLaw(ctx, store, *titles, *base)
	case "cite":
		return runCite(ctx, store, *bills, fl.Args())
	default:
		return fmt.Errorf("unknown command %q: %w", args[0], errUsage)
	}
}

func runLaw(ctx context.Context, store *fsobject.Store, titles, base string) error {
	src, err := uscode.New(uscode.Config{BaseURL: base, UserAgent: "pubcommittee-dev/0 (civic bill reader)"})
	if err != nil {
		return fmt.Errorf("law source: %w", err)
	}
	ts, err := parseTitles(titles)
	if err != nil {
		return err
	}
	res, err := lawingest.Step(ctx, src, store, store, ts)
	if err != nil {
		return fmt.Errorf("law step: %w", err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "release %s (current through %s): new=%t sections=%d\n",
		res.Release.ID, res.Release.Date.Format(time.DateOnly), res.New, res.Sections)
	return nil
}

func parseTitles(s string) ([]int, error) {
	switch s {
	case "":
		return nil, nil
	case "all":
		return lawingest.AllTitles(), nil
	}
	var out []int
	for f := range strings.SplitSeq(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return nil, fmt.Errorf("titles %q: %w", s, errUsage)
		}
		out = append(out, n)
	}
	return out, nil
}

func runCite(ctx context.Context, store *fsobject.Store, bills string, ids []string) error {
	for _, id := range ids {
		ref, err := loadBill(ctx, store, bills, id)
		if err != nil {
			return err
		}
		n, err := citations.Step(ctx, store, store, store, ref, billxml.ParserVersion, nil)
		if err != nil {
			return fmt.Errorf("citation step %s: %w", ref, err)
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s: %d versions\n", ref, n)
	}
	return nil
}

// loadBill stores every fixture version of a bill (id like "us-119-hr-4313").
func loadBill(ctx context.Context, store *fsobject.Store, dir, id string) (data.BillRef, error) {
	parts := strings.Split(id, "-")
	if len(parts) != billIDParts {
		return data.BillRef{}, fmt.Errorf("bill id %q: %w", id, errUsage)
	}
	num, err := strconv.Atoi(parts[3])
	if err != nil {
		return data.BillRef{}, fmt.Errorf("bill id %q: %w", id, errUsage)
	}
	ref := data.BillRef{Jurisdiction: parts[0], Session: parts[1], Type: parts[2], Number: num}
	files, err := fs.Glob(os.DirFS(dir), path.Join(id, "v", "*", "source.xml"))
	if err != nil || len(files) == 0 {
		return ref, fmt.Errorf("bill %s: no versions in %s: %w", id, dir, errUsage)
	}
	for _, f := range files {
		src, err := fs.ReadFile(os.DirFS(dir), f)
		if err != nil {
			return ref, fmt.Errorf("read %s: %w", f, err)
		}
		doc, err := billxml.Parse(src, nil)
		if err != nil {
			return ref, fmt.Errorf("parse %s: %w", f, err)
		}
		v := data.OfficialVersion{
			Ref: data.VersionRef{Bill: ref, Version: strings.Split(f, "/")[2]}, Type: "official",
			PublishedAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			SourceSHA256: doc.SourceSHA256, Sections: doc.DataSections(),
			Meta: data.Meta{
				Version: 1, AsOf: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Provenance: data.ProvenanceSource, ContentClass: data.ContentOfficial,
			},
		}
		if err := store.PutVersion(ctx, v, src); err != nil {
			return ref, fmt.Errorf("store %s: %w", f, err)
		}
	}
	return ref, nil
}
