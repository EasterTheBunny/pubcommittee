// Command accuracy-report measures the accuracy checks of generated content
// and writes test/accuracy/report.md ([architecture § Accuracy checks]). It
// is a development tool, not a deployed image. By default it calls no model:
// the deterministic checks and the planted-error suite run over the stored
// outputs, and every number that needs a live run reads "pending". A live
// run (--generator claude, with --grade, --grade-planted or --crs) spends
// money and needs the operator's approval and PUBCOMMITTEE_LIVE_MODEL_RUNS=1.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

const (
	generatorFake   = "fake"
	generatorClaude = "claude"
	liveSwitch      = "PUBCOMMITTEE_LIVE_MODEL_RUNS"
	// defaultSampleSize is the number of statements in the operator sample.
	defaultSampleSize = 30
)

var (
	errGenerator = errors.New("--generator must be fake or claude")
	errLive      = errors.New("--generator claude is a live model run: set " + liveSwitch + "=1 (operator approved)")
	errSameModel = errors.New("--grader-model must differ from --model")
)

// config holds the flags.
type config struct {
	Bills        []string
	Generator    string
	Model        string
	GraderModel  string
	Grade        bool
	GradePlanted bool
	CRS          bool
	SampleSize   int
	Agreement    string
	OutDir       string
	FixturesDir  string
	Args         []string
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

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "accuracy-report: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string, getenv func(string) string) (config, error) {
	fs := flag.NewFlagSet("accuracy-report", flag.ContinueOnError)
	cfg := config{Args: args}
	var bills billList
	fs.Var(&bills, "bill", "bill id (repeatable); default: the three M1 exit bills")
	fs.StringVar(&cfg.Generator, "generator", generatorFake, "fake (default) or claude (live run)")
	fs.StringVar(&cfg.Model, "model", generate.DefaultModel, "generating model whose outputs are reported")
	fs.StringVar(&cfg.GraderModel, "grader-model", generate.DefaultGraderModel, "grader model (must differ from --model)")
	fs.BoolVar(&cfg.Grade, "grade", false, "run the grounding grader on stored outputs without a grade (live with claude)")
	fs.BoolVar(&cfg.GradePlanted, "grade-planted", false, "run the grader on planted-error outputs (live with claude)")
	fs.BoolVar(&cfg.CRS, "crs", false, "compare bill summaries with the CRS summaries (live with claude)")
	fs.IntVar(&cfg.SampleSize, "sample", defaultSampleSize, "statements in the operator sample")
	fs.StringVar(&cfg.Agreement, "agreement", "", "operator-graded sample JSON to compute grader agreement from")
	fs.StringVar(&cfg.OutDir, "out", "test/accuracy", "directory for report.md and the operator sample")
	fs.StringVar(&cfg.FixturesDir, "fixtures-dir", "fixtures", "directory of the example data (BILLSTATUS files)")
	if err := fs.Parse(args); err != nil {
		return cfg, fmt.Errorf("flags: %w", err)
	}
	cfg.Bills = bills
	if len(cfg.Bills) == 0 {
		cfg.Bills = []string{"us-119-hr-4313", "us-119-hr-3062", "us-119-hr-3427"}
	}
	switch {
	case cfg.Generator != generatorFake && cfg.Generator != generatorClaude:
		return cfg, errGenerator
	case cfg.Generator == generatorClaude && getenv(liveSwitch) != "1":
		return cfg, errLive
	case cfg.GraderModel == cfg.Model:
		return cfg, errSameModel
	}
	return cfg, nil
}

func run(args []string, getenv func(string) string) error {
	cfg, err := parseArgs(args, getenv)
	if err != nil {
		return err
	}
	ports, gen, err := wire(cfg)
	if err != nil {
		return fmt.Errorf("wiring: %w", err)
	}
	gc := generate.DefaultConfig()
	gc.Model = cfg.Model
	d := generate.Deps{Ports: ports, Gen: gen, Cfg: gc, Now: time.Now, Log: slog.New(slog.NewJSONHandler(os.Stderr, nil))}
	return (&tool{cfg: cfg, d: d, gen: gen}).report(context.Background())
}

// refs parses the bill flags.
func (c config) refs() ([]data.BillRef, error) {
	out := make([]data.BillRef, 0, len(c.Bills))
	for _, id := range c.Bills {
		r, err := data.ParseBillID(id)
		if err != nil {
			return nil, fmt.Errorf("--bill: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}
