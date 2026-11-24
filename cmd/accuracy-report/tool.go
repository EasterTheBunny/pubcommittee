package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/accuracy"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// Permissions of the report directory and the files written into it.
const (
	dirMode  = 0o750
	fileMode = 0o600
)

// tool runs the steps of one report.
type tool struct {
	cfg config
	d   generate.Deps
	gen data.Generator
}

func (t *tool) live() bool { return t.cfg.Generator == generatorClaude }

// report gathers the numbers and writes report.md and the operator sample.
func (t *tool) report(ctx context.Context) error {
	bills, err := t.cfg.refs()
	if err != nil {
		return err
	}
	entries, prompts, err := t.entries(ctx, bills)
	if err != nil {
		return err
	}
	if err := t.gradeStored(ctx, entries); err != nil {
		return err
	}
	stats, err := accuracy.Collect(ctx, t.d, bills)
	if err != nil {
		return fmt.Errorf("statistics: %w", err)
	}
	in := accuracy.Input{
		At: t.d.Now(), Generator: t.cfg.Model, Live: t.live(), Bills: t.cfg.Bills, Prompts: prompts, Stats: stats,
		Planted: accuracy.RunPlanted(entries), CommandLine: "accuracy-report " + strings.Join(t.cfg.Args, " "),
	}
	if !t.live() {
		in.Generator = "fakegen"
	}
	if err := t.gradePlanted(ctx, entries, &in); err != nil {
		return err
	}
	if err := t.sample(entries, &in); err != nil {
		return err
	}
	if err := t.crsStep(ctx, bills, &in); err != nil {
		return err
	}
	return t.write("report.md", []byte(accuracy.Render(in)))
}

// gradePlanted sends the planted-error outputs to the grader when asked, and
// names the grader model in the report when a grader is part of the run.
func (t *tool) gradePlanted(ctx context.Context, entries []accuracy.SampleEntry, in *accuracy.Input) error {
	if t.cfg.GradePlanted {
		in.GraderModel = t.cfg.GraderModel
		if err := accuracy.RunPlantedGrader(ctx, t.gen, t.cfg.GraderModel, entries, in.Planted); err != nil {
			return fmt.Errorf("planted errors through the grader: %w", err)
		}
	}
	if t.cfg.Grade || t.cfg.Regrade || t.cfg.CRS {
		in.GraderModel = t.cfg.GraderModel
	}
	return nil
}

// crsStep adds the CRS comparison to the report when asked.
func (t *tool) crsStep(ctx context.Context, bills []data.BillRef, in *accuracy.Input) error {
	if !t.cfg.CRS {
		return nil
	}
	var err error
	in.CRS, err = t.crs(ctx, bills)
	return err
}

// entries loads every stored output of the bills with its context and grade,
// and the prompt version of each task.
func (t *tool) entries(ctx context.Context, bills []data.BillRef) ([]accuracy.SampleEntry, map[string]string, error) {
	var out []accuracy.SampleEntry
	prompts := map[string]string{}
	for _, b := range bills {
		tasks, _, err := generate.PlanAll(ctx, t.d, b)
		if err != nil {
			return nil, nil, fmt.Errorf("plan %s: %w", b, err)
		}
		for _, tk := range tasks {
			prompts[tk.Key.Task] = tk.Key.PromptVersion
			o, err := t.d.Ports.Generated.GetGenerated(ctx, tk.Key)
			if errors.Is(err, data.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, nil, fmt.Errorf("get %s: %w", tk.Key, err)
			}
			e := accuracy.SampleEntry{Output: o, Ctx: tk.Ctx}
			if g, err := t.d.Ports.Generated.GetGrade(ctx, tk.Key); err == nil {
				e.Grade = &g
			}
			out = append(out, e)
		}
	}
	return out, prompts, nil
}

// grade grades every ready output that has no grade yet.
func (t *tool) grade(ctx context.Context, entries []accuracy.SampleEntry) error {
	for i := range entries {
		e := &entries[i]
		if e.Output.Status != data.GenReady || e.Grade != nil {
			continue
		}
		g, err := generate.GradeOutput(ctx, t.d, t.gen, t.cfg.GraderModel, e.Output, e.Ctx)
		if err != nil {
			return fmt.Errorf("grade: %w", err)
		}
		e.Grade = &g
	}
	return nil
}

// gradeStored runs the grader runs the flags ask for on the stored outputs.
func (t *tool) gradeStored(ctx context.Context, entries []accuracy.SampleEntry) error {
	if t.cfg.Grade {
		if err := t.grade(ctx, entries); err != nil {
			return err
		}
	}
	if t.cfg.Regrade {
		return t.regrade(ctx, entries)
	}
	return nil
}

// regrade grades again every ready output whose stored grade has a
// contradicted verdict, with the current grader prompt. A new grade that the
// consistency check refuses leaves the stored grade in place; the run goes on
// and reports how many were kept.
func (t *tool) regrade(ctx context.Context, entries []accuracy.SampleEntry) error {
	var done, kept int
	for i := range entries {
		e := &entries[i]
		if e.Output.Status != data.GenReady || e.Grade == nil || !e.Grade.Contradicted() {
			continue
		}
		g, err := generate.GradeOutput(ctx, t.d, t.gen, t.cfg.GraderModel, e.Output, e.Ctx)
		switch {
		case errors.Is(err, generate.ErrOutputInvalid):
			kept++
			t.d.Log.Warn("regrade refused; stored grade kept", "output", e.Output.Key.String(), "error", err.Error())
			continue
		case err != nil:
			return fmt.Errorf("regrade: %w", err)
		}
		e.Grade = &g
		done++
	}
	t.d.Log.Info("regrade finished", "regraded", done, "kept", kept)
	return nil
}

// sample writes the operator sample and fills the agreement when the
// operator-graded file is given.
func (t *tool) sample(entries []accuracy.SampleEntry, in *accuracy.Input) error {
	items := accuracy.BuildSample(entries, t.cfg.SampleSize)
	in.SampleSize = len(items)
	body, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("sample: %w", err)
	}
	if err := t.writeSample(body, items); err != nil {
		return err
	}
	if t.cfg.Agreement == "" {
		return nil
	}
	raw, err := os.ReadFile(t.cfg.Agreement)
	if err != nil {
		return fmt.Errorf("agreement file: %w", err)
	}
	var graded []accuracy.SampleItem
	if err := json.Unmarshal(raw, &graded); err != nil {
		return fmt.Errorf("agreement file: %w", err)
	}
	// Grader labels come from the stored grades, not from the file.
	byID := map[string]string{}
	for _, it := range items {
		byID[it.ID] = it.GraderLabel
	}
	for i := range graded {
		graded[i].GraderLabel = byID[graded[i].ID]
	}
	res := accuracy.Agreement(graded)
	in.Agreement = &res
	return nil
}

// crs compares the ready summary of the first version of each bill with the
// CRS summary of the same bill (references only; never sent to the generator).
func (t *tool) crs(ctx context.Context, bills []data.BillRef) ([]accuracy.CRSRow, error) {
	var rows []accuracy.CRSRow
	for _, b := range bills {
		raw, err := os.ReadFile(filepath.Join(t.cfg.FixturesDir, "bills", b.ID(), "status.xml"))
		if err != nil {
			return nil, fmt.Errorf("status of %s: %w", b, err)
		}
		refs, err := accuracy.ParseCRSSummaries(raw)
		if err != nil || len(refs) == 0 {
			continue
		}
		vs, err := t.d.Ports.BillText.ListVersions(ctx, b)
		if err != nil || len(vs) == 0 {
			continue
		}
		row, ok, err := t.crsRow(ctx, vs[0].Ref, refs[0].Text)
		if err != nil {
			return nil, err
		}
		if ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (t *tool) crsRow(ctx context.Context, v data.VersionRef, reference string) (accuracy.CRSRow, bool, error) {
	o, err := t.d.Ports.Generated.GetCurrent(ctx, data.TaskBillSummary, v, "")
	if errors.Is(err, data.ErrNotFound) {
		return accuracy.CRSRow{}, false, nil
	}
	if err != nil {
		return accuracy.CRSRow{}, false, fmt.Errorf("summary of %s: %w", v, err)
	}
	texts := []string{o.Headline}
	for _, s := range o.Statements {
		texts = append(texts, s.Text)
	}
	req, err := accuracy.CRSRequest("crs-"+o.Key.ID(), strings.Join(texts, "\n"), reference, t.cfg.GraderModel)
	if err != nil {
		return accuracy.CRSRow{}, false, fmt.Errorf("crs request: %w", err)
	}
	res, err := t.gen.Generate(ctx, req)
	if err != nil {
		return accuracy.CRSRow{}, false, fmt.Errorf("crs %s: %w", v, err)
	}
	points, err := accuracy.ParseCRS(res.Text)
	if err != nil {
		return accuracy.CRSRow{}, false, fmt.Errorf("crs %s: %w", v, err)
	}
	total, covered, contradicts := accuracy.Coverage(points)
	return accuracy.CRSRow{Version: v, Points: total, Covered: covered, Contradicts: contradicts}, true, nil
}

// writeSample writes the operator sample files, unless --report-only keeps
// an earlier sample (graded by the operator) in place.
func (t *tool) writeSample(body []byte, items []accuracy.SampleItem) error {
	if t.cfg.ReportOnly {
		return nil
	}
	if err := t.write("operator-sample.json", body); err != nil {
		return err
	}
	return t.write("operator-sample.md", []byte(accuracy.SampleMarkdown(items)))
}

func (t *tool) write(name string, body []byte) error {
	if err := os.MkdirAll(t.cfg.OutDir, dirMode); err != nil {
		return fmt.Errorf("out dir: %w", err)
	}
	path := filepath.Join(t.cfg.OutDir, name)
	if err := os.WriteFile(path, body, fileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// hardSampleMix is the mix of the hard sample: every contradicted statement
// (the population is small), the most informative validator-rejected ones,
// and as many supported controls as contradicted, so the order and the mix do
// not tell the operator the answer.
// hardPerStratum is the size of each stratum of the hard sample (30 in all).
const hardPerStratum = 10

func hardSampleMix() accuracy.HardMix {
	return accuracy.HardMix{Contradicted: hardPerStratum, Rejected: hardPerStratum, Control: hardPerStratum}
}

// hardSampleRun loads the stored outputs and writes the hard sample only.
func (t *tool) hardSampleRun(ctx context.Context) error {
	bills, err := t.cfg.refs()
	if err != nil {
		return err
	}
	entries, _, err := t.entries(ctx, bills)
	if err != nil {
		return err
	}
	return t.hardSample(entries)
}

// hardSample writes the hard operator sample and nothing else, so report.md
// and the first sample stay as they are.
func (t *tool) hardSample(entries []accuracy.SampleEntry) error {
	exclude := map[string]bool{}
	if t.cfg.ExcludeFile != "" {
		raw, err := os.ReadFile(t.cfg.ExcludeFile)
		if err != nil {
			return fmt.Errorf("exclude file: %w", err)
		}
		var prev []accuracy.SampleItem
		if err := json.Unmarshal(raw, &prev); err != nil {
			return fmt.Errorf("exclude file: %w", err)
		}
		for _, it := range prev {
			exclude[it.ID] = true
		}
	}
	items := accuracy.BuildHardSample(entries, hardSampleMix(), exclude)
	body, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("hard sample: %w", err)
	}
	if err := t.write(t.cfg.HardSample+".json", body); err != nil {
		return err
	}
	return t.write(t.cfg.HardSample+".md", []byte(accuracy.SampleMarkdown(items)))
}

// panelRun checks the statements of a sample with the panel models and
// writes <sample>.panel.md and .json next to it, and nothing else.
func (t *tool) panelRun(ctx context.Context) error {
	raw, err := os.ReadFile(t.cfg.PanelFile)
	if err != nil {
		return fmt.Errorf("panel sample: %w", err)
	}
	var sample []accuracy.SampleItem
	if err := json.Unmarshal(raw, &sample); err != nil {
		return fmt.Errorf("panel sample: %w", err)
	}
	bills, err := t.cfg.refs()
	if err != nil {
		return err
	}
	entries, _, err := t.entries(ctx, bills)
	if err != nil {
		return err
	}
	items := accuracy.RunPanel(ctx, t.gen, sample, entries, t.cfg.PanelModels)
	body, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("panel: %w", err)
	}
	base := strings.TrimSuffix(t.cfg.PanelFile, ".json") + ".panel"
	if err := os.WriteFile(base+".json", body, fileMode); err != nil {
		return fmt.Errorf("write panel: %w", err)
	}
	md := accuracy.PanelMarkdown(items, t.cfg.GraderModel, t.cfg.Model)
	if err := os.WriteFile(base+".md", []byte(md), fileMode); err != nil {
		return fmt.Errorf("write panel: %w", err)
	}
	t.d.Log.Info("panel written", "file", base+".md", "statements", len(items))
	return nil
}
