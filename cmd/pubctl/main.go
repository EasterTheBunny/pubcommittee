// Command pubctl is the operator CLI for admin-api: it calls admin-api over
// HTTP and prints each answer as JSON. Mutating commands need a --reason,
// which is published in the public log. A loopback --url needs no token; in
// the cloud the token comes from PUBCTL_TOKEN or `gcloud auth print-identity-token`.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

const (
	envURL         = "PUBCTL_URL"
	dateLayout     = "2006-01-02"
	builtinV1      = "v1"
	actionTrack    = "track"
	actionReingest = "reingest"
)

var (
	errReasonRequired = errors.New("--reason is required: it is published in the public log")
	errURLRequired    = errors.New("set --url or PUBCTL_URL")
	errUsage          = errors.New("usage")
	errUnknownCommand = errors.New("unknown command")
	errNoPatch        = errors.New("catalog patch needs at least one catalog flag, such as --title")
	errPullFailed     = errors.New("the pull reported failures")
)

const usageText = `usage: pubctl [--url <base>] <command> [flags]

--url defaults to $PUBCTL_URL. A <bill> looks like us-119-hr-4313.
Mutating commands require --reason; the reason is published.

commands:
  bills track <bill> --reason R
  bills untrack <bill> --reason R
  bills catalog patch <bill> --reason R [--title T] [--status S] [--chamber C]
      [--introduced YYYY-MM-DD] [--policy-area A] [--subjects a,b,c]
  bills reingest <bill> --reason R
  bills reparse <bill> --version N --reason R
  generations create --task T --bill <bill> --version N --reason R
  selection-policy publish (--builtin v1 | --file path.json) --reason R
  ingest failures [--source <source/session>]...
  jobs
`

// main runs pubctl with the process arguments and standard streams.
func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "pubctl: %v\n", err)
		os.Exit(1)
	}
}

// cli holds what every command needs: the admin-api base URL and the streams.
type cli struct {
	url    string
	getenv func(string) string
	out    io.Writer
	errOut io.Writer
}

// run parses the global flags and dispatches one command. -h prints help and
// returns nil.
func run(ctx context.Context, args []string, getenv func(string) string, out, errOut io.Writer) error {
	global := flag.NewFlagSet("pubctl", flag.ContinueOnError)
	global.SetOutput(errOut)
	global.Usage = func() { printUsage(errOut) }
	base := global.String("url", "", "admin-api base URL (default $PUBCTL_URL)")
	if err := global.Parse(args); err != nil {
		return ignoreHelp(err)
	}
	rest := global.Args()
	if len(rest) == 0 {
		printUsage(errOut)
		return fmt.Errorf("no command: %w", errUsage)
	}
	c := cli{url: *base, getenv: getenv, out: out, errOut: errOut}
	return c.dispatch(ctx, rest[0], rest[1:])
}

// dispatch routes one command name to its handler.
func (c cli) dispatch(ctx context.Context, name string, args []string) error {
	switch name {
	case "bills":
		return c.bills(ctx, args)
	case "generations":
		return c.generations(ctx, args)
	case "selection-policy":
		return c.selectionPolicy(ctx, args)
	case "ingest":
		return c.ingest(ctx, args)
	case "jobs":
		return c.jobs(ctx, args)
	default:
		printUsage(c.errOut)
		return fmt.Errorf("%w %q", errUnknownCommand, name)
	}
}

// bills dispatches the bills sub-commands.
func (c cli) bills(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage(c.errOut)
		return fmt.Errorf("bills needs a sub-command: %w", errUsage)
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "track", "untrack", "reingest":
		return c.billAction(ctx, sub, rest)
	case "reparse":
		return c.reparse(ctx, rest)
	case "catalog":
		if len(rest) == 0 || rest[0] != "patch" {
			return fmt.Errorf("bills catalog needs sub-command patch: %w", errUsage)
		}
		return c.catalogPatch(ctx, rest[1:])
	default:
		printUsage(c.errOut)
		return fmt.Errorf("%w %q", errUnknownCommand, "bills "+sub)
	}
}

// billAction sends track, untrack, or reingest for one bill.
func (c cli) billAction(ctx context.Context, action string, args []string) error {
	fs := c.flagSet("bills " + action)
	reason := fs.String("reason", "", "public reason (required)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return ignoreHelp(err)
	}
	ref, err := billPositional(pos)
	if err != nil {
		return err
	}
	if err := requireReason(*reason); err != nil {
		return err
	}
	cl, err := c.connect()
	if err != nil {
		return err
	}
	var res adminapi.ActionResult
	path := adminapi.PathPrefix + "/bills/" + ref.ID() + "/" + action
	if err := cl.do(ctx, http.MethodPost, path, adminapi.Reasoned{Reason: *reason}, &res); err != nil {
		return err
	}
	if err := c.print(res); err != nil {
		return err
	}
	if (action == actionTrack || action == actionReingest) && pullFailed(res) {
		return errPullFailed
	}
	return nil
}

// reparse sends a reparse of one stored version of a bill.
func (c cli) reparse(ctx context.Context, args []string) error {
	fs := c.flagSet("bills reparse")
	reason := fs.String("reason", "", "public reason (required)")
	version := fs.Int("version", 0, "stored version number, 1 or more (required)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return ignoreHelp(err)
	}
	ref, err := billPositional(pos)
	if err != nil {
		return err
	}
	if err := requireReason(*reason); err != nil {
		return err
	}
	if *version < 1 {
		return fmt.Errorf("--version must be 1 or more: %w", errUsage)
	}
	cl, err := c.connect()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	path := fmt.Sprintf("%s/bills/%s/v/%d/reparse", adminapi.PathPrefix, ref.ID(), *version)
	if err := cl.do(ctx, http.MethodPost, path, adminapi.Reasoned{Reason: *reason}, &raw); err != nil {
		return err
	}
	return c.print(raw)
}

// catalogPatch sends the catalog fields that were given on the command line.
func (c cli) catalogPatch(ctx context.Context, args []string) error {
	fs := c.flagSet("bills catalog patch")
	reason := fs.String("reason", "", "public reason (required)")
	title := fs.String("title", "", "catalog title")
	status := fs.String("status", "", "bill status")
	chamber := fs.String("chamber", "", "chamber")
	introduced := fs.String("introduced", "", "introduced date, YYYY-MM-DD")
	policyArea := fs.String("policy-area", "", "policy area")
	subjects := fs.String("subjects", "", "comma-separated subjects")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return ignoreHelp(err)
	}
	ref, err := billPositional(pos)
	if err != nil {
		return err
	}
	if err := requireReason(*reason); err != nil {
		return err
	}
	patch, err := buildPatch(fs, *reason, catalogFlags{
		title: title, status: status, chamber: chamber, introduced: introduced,
		policyArea: policyArea, subjects: subjects,
	})
	if err != nil {
		return err
	}
	cl, err := c.connect()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	path := adminapi.PathPrefix + "/bills/" + ref.ID() + "/catalog"
	if err := cl.do(ctx, http.MethodPatch, path, patch, &raw); err != nil {
		return err
	}
	return c.print(raw)
}

// catalogFlags are the catalog flag values, read after parsing.
type catalogFlags struct {
	title, status, chamber, introduced, policyArea, subjects *string
}

// buildPatch sets only the catalog fields whose flags were visited.
func buildPatch(fs *flag.FlagSet, reason string, f catalogFlags) (adminapi.CatalogPatch, error) {
	var patch adminapi.CatalogPatch
	patch.Reason = reason
	set := 0
	var err error
	fs.Visit(func(fl *flag.Flag) {
		if fl.Name == "reason" {
			return
		}
		set++
		if err == nil {
			err = applyFlag(&patch, fl.Name, f)
		}
	})
	if err != nil {
		return adminapi.CatalogPatch{}, err
	}
	if set == 0 {
		return adminapi.CatalogPatch{}, errNoPatch
	}
	return patch, nil
}

// applyFlag copies one visited catalog flag into patch.
func applyFlag(patch *adminapi.CatalogPatch, name string, f catalogFlags) error {
	switch name {
	case "title":
		patch.Title = f.title
	case "status":
		patch.Status = f.status
	case "chamber":
		patch.Chamber = f.chamber
	case "policy-area":
		patch.PolicyArea = f.policyArea
	case "introduced":
		if _, err := time.Parse(dateLayout, *f.introduced); err != nil {
			return fmt.Errorf("--introduced %q must be YYYY-MM-DD: %w", *f.introduced, errUsage)
		}
		patch.Introduced = f.introduced
	case "subjects":
		list := splitList(*f.subjects)
		patch.Subjects = &list
	}
	return nil
}

// splitList splits a comma-separated flag value and drops empty items.
func splitList(s string) []string {
	list := []string{}
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}

// generations dispatches generations create.
func (c cli) generations(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("generations needs sub-command create: %w", errUsage)
	}
	fs := c.flagSet("generations create")
	task := fs.String("task", "", "generation task name (required)")
	bill := fs.String("bill", "", "bill id, for example us-119-hr-4313 (required)")
	version := fs.Int("version", 0, "stored version number, 1 or more (required)")
	reason := fs.String("reason", "", "public reason (required)")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return ignoreHelp(err)
	}
	if len(pos) != 0 {
		return fmt.Errorf("generations create takes flags only: %w", errUsage)
	}
	body, err := generationRequest(*task, *bill, *version, *reason)
	if err != nil {
		return err
	}
	cl, err := c.connect()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := cl.do(ctx, http.MethodPost, adminapi.GenerationRoute, body, &raw); err != nil {
		return err
	}
	return c.print(raw)
}

// selectionPolicy dispatches selection-policy publish.
func (c cli) selectionPolicy(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "publish" {
		return fmt.Errorf("selection-policy needs sub-command publish: %w", errUsage)
	}
	fs := c.flagSet("selection-policy publish")
	builtin := fs.String("builtin", "", "built-in policy to publish: v1")
	file := fs.String("file", "", "path to a policy JSON file")
	reason := fs.String("reason", "", "public reason (required)")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return ignoreHelp(err)
	}
	if len(pos) != 0 {
		return fmt.Errorf("selection-policy publish takes flags only: %w", errUsage)
	}
	if err := requireReason(*reason); err != nil {
		return err
	}
	policy, err := policyDocument(*builtin, *file)
	if err != nil {
		return err
	}
	cl, err := c.connect()
	if err != nil {
		return err
	}
	var body adminapi.PolicyRequest
	body.Reason = *reason
	body.Policy = policy
	var raw json.RawMessage
	if err := cl.do(ctx, http.MethodPost, adminapi.PolicyRoute, body, &raw); err != nil {
		return err
	}
	return c.print(raw)
}

// generationRequest validates the flags of generations create.
func generationRequest(task, bill string, version int, reason string) (adminapi.GenerationRequest, error) {
	if err := requireReason(reason); err != nil {
		return adminapi.GenerationRequest{}, err
	}
	ref, err := data.ParseBillID(bill)
	if err != nil {
		return adminapi.GenerationRequest{}, fmt.Errorf("--bill: %w", err)
	}
	if task == "" || version < 1 {
		return adminapi.GenerationRequest{}, fmt.Errorf("--task and --version (1 or more) are required: %w", errUsage)
	}
	var body adminapi.GenerationRequest
	body.Reason = reason
	body.Task = task
	body.Bill = ref.ID()
	body.Version = version
	return body, nil
}

// policyDocument returns the policy JSON from exactly one of --builtin or --file.
func policyDocument(builtin, file string) (json.RawMessage, error) {
	switch {
	case builtin != "" && file != "":
		return nil, fmt.Errorf("give one of --builtin or --file, not both: %w", errUsage)
	case builtin != "":
		return builtinPolicy(builtin)
	case file != "":
		return policyFile(file)
	default:
		return nil, fmt.Errorf("give --builtin v1 or --file path.json: %w", errUsage)
	}
}

// builtinPolicy encodes the built-in selection policy named by name.
func builtinPolicy(name string) (json.RawMessage, error) {
	if name != builtinV1 {
		return nil, fmt.Errorf("--builtin must be %q, got %q: %w", builtinV1, name, errUsage)
	}
	pol, err := selection.V1(time.Now())
	if err != nil {
		return nil, fmt.Errorf("built-in policy %s: %w", name, err)
	}
	raw, err := json.Marshal(pol)
	if err != nil {
		return nil, fmt.Errorf("encode built-in policy: %w", err)
	}
	return raw, nil
}

// policyFile reads a policy document and checks that it is JSON.
func policyFile(path string) (json.RawMessage, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read --file: %w", err)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("--file %q is not valid JSON: %w", path, errUsage)
	}
	return raw, nil
}

// ingest dispatches ingest failures.
func (c cli) ingest(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "failures" {
		return fmt.Errorf("ingest needs sub-command failures: %w", errUsage)
	}
	fs := c.flagSet("ingest failures")
	var sources multiFlag
	fs.Var(&sources, "source", "source/session to show; repeat for more")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return ignoreHelp(err)
	}
	if len(pos) != 0 {
		return fmt.Errorf("ingest failures takes flags only: %w", errUsage)
	}
	path := adminapi.FailuresRoute
	if len(sources) > 0 {
		q := url.Values{}
		for _, s := range sources {
			q.Add("source", s)
		}
		path += "?" + q.Encode()
	}
	return c.get(ctx, path)
}

// jobs sends GET /jobs.
func (c cli) jobs(ctx context.Context, args []string) error {
	fs := c.flagSet("jobs")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return ignoreHelp(err)
	}
	if len(pos) != 0 {
		return fmt.Errorf("jobs takes no arguments: %w", errUsage)
	}
	return c.get(ctx, adminapi.JobsRoute)
}

// get sends a read-only GET and prints the answer.
func (c cli) get(ctx context.Context, path string) error {
	cl, err := c.connect()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := cl.do(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return err
	}
	return c.print(raw)
}

// connect builds the admin-api client from --url or PUBCTL_URL.
func (c cli) connect() (*client, error) {
	base := c.url
	if base == "" {
		base = c.getenv(envURL)
	}
	if base == "" {
		return nil, errURLRequired
	}
	return newClient(base, c.getenv, nil)
}

// print writes v as indented JSON followed by a newline.
func (c cli) print(v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	if _, err := fmt.Fprintln(c.out, string(raw)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// flagSet makes a FlagSet for one command whose -h help goes to errOut.
func (c cli) flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.errOut)
	return fs
}

// multiFlag is a repeatable string flag.
type multiFlag []string

// String implements flag.Value.
func (m *multiFlag) String() string { return strings.Join(*m, ",") }

// Set implements flag.Value.
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// parseArgs parses flags that may follow positional arguments, as in
// `bills track <bill> --reason R`. It returns the positional arguments in order.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, fmt.Errorf("parse flags: %w", err)
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// billPositional returns the one bill id given as a positional argument.
func billPositional(pos []string) (data.BillRef, error) {
	if len(pos) != 1 {
		return data.BillRef{}, fmt.Errorf("expected one bill id, got %d: %w", len(pos), errUsage)
	}
	ref, err := data.ParseBillID(pos[0])
	if err != nil {
		return data.BillRef{}, fmt.Errorf("bill: %w", err)
	}
	return ref, nil
}

// requireReason refuses an empty reason before any request is sent.
func requireReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errReasonRequired
	}
	return nil
}

// pullFailed reports whether a track or reingest left its pull unfinished or failed.
func pullFailed(res adminapi.ActionResult) bool {
	return res.PullError != "" || (res.Pull != nil && res.Pull.Failed())
}

// ignoreHelp turns -h into a nil error; the help text is already printed.
func ignoreHelp(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

// printUsage writes the command list to w.
func printUsage(w io.Writer) {
	_, _ = io.WriteString(w, usageText)
}
