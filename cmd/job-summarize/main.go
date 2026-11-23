// Command job-summarize generates bill summaries, change explanations and
// version change summaries with Claude, as a two-phase job (M1-09): --submit
// plans the tasks of the named bills and submits one batch per bill, a later
// --collect run validates and stores the finished batches. Nothing chains the
// two, and nothing chains this job to job-ingest. It prints one JSON report
// per bill or batch run. See cmd/job-summarize/README.md.
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
	"github.com/easterthebunny/pubcommittee/internal/generate"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

const serviceName = "job-summarize"

// Generator names of --generator.
const (
	generatorFake   = "fake"
	generatorClaude = "claude"
)

// liveSwitch is the environment variable that must be 1 for --generator
// claude: live model runs are off unless the operator turns them on.
const liveSwitch = "PUBCOMMITTEE_LIVE_MODEL_RUNS"

var (
	errMode      = errors.New("exactly one of --submit or --collect is required")
	errNoBill    = errors.New("--submit needs at least one --bill <id>, for example --bill us-119-hr-4313")
	errGenerator = errors.New("--generator must be fake or claude")
	errLive      = errors.New("--generator claude is a live model run: set " + liveSwitch + "=1 (operator approved)")
	errFailed    = errors.New("one or more bills failed")
)

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

// config is what the wiring needs to build the generator.
type config struct {
	Submit    bool
	Collect   bool
	DryRun    bool
	Generator string
	Model     string
	Live      bool
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

// validate checks the flag combination; bills is the number of --bill values.
func (c config) validate(bills int) error {
	switch {
	case c.Submit == c.Collect:
		return errMode
	case c.Generator != generatorFake && c.Generator != generatorClaude:
		return errGenerator
	case c.Generator == generatorClaude && !c.Live:
		return errLive
	case c.Submit && bills == 0:
		return errNoBill
	}
	return nil
}

func parseArgs(args []string, getenv func(string) string) (config, []data.BillRef, error) {
	fs := flag.NewFlagSet(serviceName, flag.ContinueOnError)
	var bills billList
	cfg := config{}
	fs.Var(&bills, "bill", "bill id, for example us-119-hr-4313 (repeatable, or comma separated)")
	fs.BoolVar(&cfg.Submit, "submit", false, "plan the tasks of the bills and submit the ones with no stored result")
	fs.BoolVar(&cfg.Collect, "collect", false, "collect finished batches: validate, store, publish")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "with --submit: plan and count; submit and write nothing")
	fs.StringVar(&cfg.Generator, "generator", generatorFake, "fake (default) or claude (live run)")
	fs.StringVar(&cfg.Model, "model", generate.DefaultModel, "generating model id")
	if err := fs.Parse(args); err != nil {
		return cfg, nil, fmt.Errorf("flags: %w", err)
	}
	cfg.Live = getenv(liveSwitch) == "1"
	if err := cfg.validate(len(bills)); err != nil {
		return cfg, nil, err
	}
	refs := make([]data.BillRef, 0, len(bills))
	for _, id := range bills {
		ref, err := data.ParseBillID(id)
		if err != nil {
			return cfg, nil, fmt.Errorf("--bill: %w", err)
		}
		refs = append(refs, ref)
	}
	return cfg, refs, nil
}

// run parses the flags, wires the ports and runs one phase.
func run(args []string, getenv func(string) string, out io.Writer) error {
	started := time.Now()
	cfg, refs, err := parseArgs(args, getenv)
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
	ports, gen, err := wire(tel, cfg)
	if err != nil {
		return fmt.Errorf("wiring: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	gc := generate.DefaultConfig()
	gc.Model = cfg.Model
	deps := generate.Deps{
		Ports: ports, Gen: gen, Cfg: gc, Now: time.Now, Log: logger,
		Record: func(ctx context.Context, task, model, status string, in, outTok int) {
			tel.Instruments.RecordGeneration(ctx, task, model, status, in, outTok)
		},
	}
	var items int
	var runErr error
	if cfg.Submit {
		items, runErr = submitAll(context.Background(), deps, refs, cfg.DryRun, out)
	} else {
		items, runErr = collect(context.Background(), deps, out)
	}
	tel.Instruments.RecordJob(context.Background(), serviceName, started, items, runErr)
	if err := tel.Shutdown(context.Background()); err != nil {
		logger.Warn("telemetry shutdown", "error", err.Error())
	}
	return runErr
}

func submitAll(ctx context.Context, d generate.Deps, refs []data.BillRef, dry bool, out io.Writer) (int, error) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	items, failed := 0, false
	for _, ref := range refs {
		rep, err := generate.Submit(ctx, d, ref, dry)
		if encErr := enc.Encode(rep); encErr != nil {
			return items, fmt.Errorf("report: %w", encErr)
		}
		if err != nil {
			d.Log.Error("submit failed", "bill", ref.ID(), "error", err.Error())
			failed = true
		}
		items += rep.Submitted
	}
	if failed {
		return items, errFailed
	}
	return items, nil
}

func collect(ctx context.Context, d generate.Deps, out io.Writer) (int, error) {
	rep, err := generate.Collect(ctx, d)
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(rep); encErr != nil {
		return 0, fmt.Errorf("report: %w", encErr)
	}
	items := 0
	for _, n := range rep.ByStatus {
		items += n
	}
	if err != nil {
		return items, fmt.Errorf("collect: %w", err)
	}
	return items, nil
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}
