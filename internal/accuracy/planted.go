package accuracy

import (
	"context"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// NotRun marks a grader count that was not measured.
const NotRun = -1

// RunPlanted plants every kind of error into every ready output and counts
// how many the deterministic check meant for it catches. It calls no model,
// so the catch rates of the deterministic checks are real numbers on any
// outputs, including fakegen's. CaughtByGrade is NotRun.
func RunPlanted(entries []SampleEntry) []PlantRow {
	rows := make([]PlantRow, 0, len(generate.PlantKinds()))
	for _, kind := range generate.PlantKinds() {
		row := PlantRow{Kind: kind, Check: generate.CheckOf(kind), CaughtByGrade: NotRun}
		for _, e := range entries {
			if e.Output.Status != data.GenReady {
				continue
			}
			planted, ok := generate.Plant(kind, e.Output, e.Ctx)
			if !ok {
				continue
			}
			row.Planted++
			findings := generate.Validate(e.Output.Key.Task, generate.ModelOutput{
				Headline: planted.Headline, Statements: planted.Statements,
			}, e.Ctx)
			if row.Check != "" && generate.Caught(row.Check, findings) {
				row.CaughtByCheck++
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// RunPlantedGrader sends every planted output to the grader and counts those
// where it marks any statement unsupported or contradicted, or the wording
// not neutral. It fills CaughtByGrade of rows (as returned by RunPlanted).
// It is a live run when gen is a model: callers only reach it from the
// accuracy report with operator approval, never from CI.
func RunPlantedGrader(
	ctx context.Context, gen data.Generator, graderModel string, entries []SampleEntry, rows []PlantRow,
) error {
	for i := range rows {
		rows[i].CaughtByGrade = 0
		for _, e := range entries {
			if e.Output.Status != data.GenReady {
				continue
			}
			planted, ok := generate.Plant(rows[i].Kind, e.Output, e.Ctx)
			if !ok {
				continue
			}
			caught, err := gradedBad(ctx, gen, graderModel, planted, e.Ctx)
			if err != nil {
				return err
			}
			if caught {
				rows[i].CaughtByGrade++
			}
		}
	}
	return nil
}

func gradedBad(
	ctx context.Context, gen data.Generator, graderModel string, o data.GeneratedOutput, gc data.GenContext,
) (bool, error) {
	req, err := generate.GradeRequest(o, gc, graderModel)
	if err != nil {
		return false, fmt.Errorf("planted grade request: %w", err)
	}
	res, err := gen.Generate(ctx, req)
	if err != nil {
		return false, fmt.Errorf("planted grade %s: %w", o.Key, err)
	}
	if res.Outcome != "ok" {
		return false, nil
	}
	g, err := generate.ParseGrade(res.Text, generate.GradeCited(o, gc))
	return caughtBy(g, err == nil), nil
}

// caughtBy reports whether a grade marks the planted output bad. An unreadable
// grade (ok false) counts as not caught.
func caughtBy(g data.Grade, ok bool) bool {
	if !ok {
		return false
	}
	if !g.Neutral {
		return true
	}
	for _, s := range g.Statements {
		if s.Label != data.GradeSupported {
			return true
		}
	}
	return false
}
