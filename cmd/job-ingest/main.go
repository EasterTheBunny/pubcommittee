// Command job-ingest pulls bills from the official source on demand (M1:
// manual, never scheduled). Each run pulls the bills named with --bill: the
// catalog entry always, and for a tracked bill every official text version
// not yet stored, parsed in the run. It prints one JSON report per bill and
// exits non-zero when any pull failed. See cmd/job-ingest/README.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

const serviceName = "job-ingest"

// userAgent is sent to the Law Revision Counsel on every target; neutral, no
// environment or deployment names.
const userAgent = "pubcommittee-ingest/0 (open civic project; existing-law pull)"

// Source names of --source.
const (
	sourceFixtures = "fixtures"
	sourceGovinfo  = "govinfo"
)

var (
	errNoBill = errors.New("at least one --bill <id> is required, for example --bill us-119-hr-4313")
	errSource = errors.New("--source must be fixtures or govinfo")
	errFailed = errors.New("one or more pulls failed")
)

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

// config is what the wiring needs to build the bill source.
type config struct {
	Source      string // sourceFixtures or sourceGovinfo
	FixturesDir string
	// Evaluate runs the selection policy over the stored catalog.
	Evaluate bool
	// PublishPolicy is "v1" or a policy file to store as a policy version.
	PublishPolicy string
	PolicyReason  string
	// Law runs the existing-law ingest (M1-07) instead of a bill pull; Titles
	// names the US Code titles to store ("all", or 3,42; fixtures: the titles
	// the fixtures hold when empty).
	Law bool
	// Analyze runs the citation and comparison steps over stored data: the
	// bills named, or every tracked bill in the catalog. Fetches nothing.
	Analyze bool
	Titles  string
}

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", serviceName, err)
		os.Exit(1)
	}
}

type billList []string

func (l *billList) String() string { return strings.Join(*l, ",") }

func (l *billList) Set(v string) error {
	for p := range strings.SplitSeq(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*l = append(*l, p)
		}
	}
	return nil
}

// parseArgs reads the flags into the config, the pull options and the bill list.
// needsBill is true for the modes that name no bill themselves: a pull does.
func (c config) needsBill() bool {
	return !c.Evaluate && c.PublishPolicy == "" && !c.Law && !c.Analyze
}

func parseArgs(args []string, getenv func(string) string) (config, ingest.Options, []data.BillRef, error) {
	fs := flag.NewFlagSet(serviceName, flag.ContinueOnError)
	var bills billList
	var opts ingest.Options
	cfg := config{}
	fs.Var(&bills, "bill", "bill id, for example us-119-hr-4313 (repeatable, or comma separated)")
	fs.StringVar(&cfg.Source, "source", envOr(getenv, "INGEST_SOURCE", defaultSource),
		"fixtures or govinfo (cloud targets: govinfo only, the default)")
	fs.StringVar(&cfg.FixturesDir, "fixtures-dir", envOr(getenv, "FIXTURES_DIR", "fixtures"),
		"directory of the example data")
	fs.BoolVar(&opts.Track, "track", false, "track the bill: fetch, parse and store every text version")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "report what would change; write nothing, fetch no text")
	fs.BoolVar(&cfg.Evaluate, "evaluate", false,
		"evaluate the stored catalog against the latest selection policy; writes changed entries and the breakdown")
	fs.StringVar(&cfg.PublishPolicy, "publish-policy", "",
		"store a selection policy version: 'v1' (built in) or a JSON file; local target only")
	fs.StringVar(&cfg.PolicyReason, "reason", "initial selection policy", "public reason logged with --publish-policy")
	addDataFlags(fs, &cfg)
	fs.BoolVar(&opts.Verify, "verify", false, "re-fetch stored originals and compare SHA-256; never overwrites")
	if err := fs.Parse(args); err != nil {
		return cfg, opts, nil, fmt.Errorf("flags: %w", err)
	}
	if len(bills) == 0 && cfg.needsBill() {
		return cfg, opts, nil, errNoBill
	}
	if cfg.Source != sourceFixtures && cfg.Source != sourceGovinfo {
		return cfg, opts, nil, errSource
	}
	if err := checkSource(cfg); err != nil {
		return cfg, opts, nil, err
	}
	refs := make([]data.BillRef, 0, len(bills))
	for _, id := range bills {
		ref, err := data.ParseBillID(id)
		if err != nil {
			return cfg, opts, nil, fmt.Errorf("--bill: %w", err)
		}
		refs = append(refs, ref)
	}
	return cfg, opts, refs, nil
}

// addDataFlags registers the existing-law and stored-data modes.
func addDataFlags(fs *flag.FlagSet, cfg *config) {
	fs.BoolVar(&cfg.Law, "law", false,
		"existing-law ingest: store the newest US Code release point (a release already stored is left alone)")
	fs.StringVar(&cfg.Titles, "titles", "", "with --law: comma-separated title numbers, or all")
	fs.BoolVar(&cfg.Analyze, "analyze", false,
		"run citations and comparisons over stored versions (the --bill bills, or every tracked bill); fetches nothing")
}

// run parses the flags, wires the ports and runs one pull, evaluation or policy publication.
func run(args []string, getenv func(string) string, out io.Writer) error {
	started := time.Now()
	cfg, opts, refs, err := parseArgs(args, getenv)
	if err != nil {
		return err
	}

	tel, err := telemetry.Setup(telemetry.Options{
		Kind: telemetry.KindJob, ServiceName: serviceName, Version: version,
		Environment: envOr(getenv, "DEPLOYMENT_ENVIRONMENT", "dev"),
	})
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	ports, fetcher, err := wire(tel, cfg)
	if err != nil {
		return fmt.Errorf("wiring: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	deps := ingest.Deps{Ports: ports, Fetcher: fetcher, Log: logger, Now: time.Now}
	var items int
	var runErr error
	switch {
	case cfg.Analyze:
		items, runErr = analyzeStored(context.Background(), refs, deps, opts, out)
	case cfg.Law:
		items, runErr = ingestLaw(context.Background(), deps, fetcher, cfg, opts, out)
	case cfg.PublishPolicy != "":
		runErr = publishPolicy(context.Background(), deps, cfg, out)
	case cfg.Evaluate:
		items, runErr = evaluateCatalog(context.Background(), deps, opts, out)
	default:
		items, runErr = pullAll(context.Background(), refs, deps, cfg, opts, out)
	}
	tel.Instruments.RecordJob(context.Background(), serviceName, started, items, runErr)
	if err := tel.Shutdown(context.Background()); err != nil {
		logger.Warn("telemetry shutdown", "error", err.Error())
	}
	return runErr
}

func pullAll(
	ctx context.Context, refs []data.BillRef, d ingest.Deps, cfg config, opts ingest.Options, out io.Writer,
) (int, error) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	failed, items := false, 0
	steps, policy, err := ingest.PullSteps(ctx, d)
	if err != nil {
		return 0, fmt.Errorf("pull steps: %w", err)
	}
	for _, ref := range refs {
		d.Source = cfg.Source + "/" + ref.Session
		rep, err := ingest.Pull(ctx, d, opts, steps, ref)
		if encErr := enc.Encode(rep); encErr != nil {
			return items, fmt.Errorf("report: %w", encErr)
		}
		if err != nil {
			d.Log.Error("pull failed", "bill", ref.ID(), "error", err.Error())
		}
		if err != nil || rep.Failed() {
			failed = true
		}
		items += len(rep.Stored)
	}
	if err := publishBreakdown(ctx, d, policy, opts); err != nil {
		return items, err
	}
	if failed {
		return items, errFailed
	}
	return items, nil
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}
