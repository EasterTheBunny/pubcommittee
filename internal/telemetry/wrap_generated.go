package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// RecordGeneration records the tokens of one model call
// (gen_ai.client.token.usage, by token type and model) and counts its
// outcome (pubcommittee.generation.results, by task and outcome). Outcomes
// are the Status values of data.GeneratedOutput.
func (i *Instruments) RecordGeneration(ctx context.Context, task, model, outcome string, tokensIn, tokensOut int) {
	for typ, n := range map[string]int{"input": tokensIn, "output": tokensOut} {
		i.genTokens.Record(ctx, int64(n), metric.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.request.model", model),
			attribute.String("gen_ai.token.type", typ),
			attribute.String("pubcommittee.task", task),
		))
	}
	i.genResults.Add(ctx, 1, metric.WithAttributes(
		attribute.String("pubcommittee.task", task),
		attribute.String("pubcommittee.outcome", outcome),
	))
}

// wrapGeneratedReader keeps a nil reader nil: an adapter that stores no
// generations leaves the port unset and readers answer not found.
func wrapGeneratedReader(t timer, n data.GeneratedReader) data.GeneratedReader {
	if n == nil {
		return nil
	}
	return timedGeneratedReader{t, n}
}

func wrapGeneratedWriter(t timer, n data.GeneratedWriter) data.GeneratedWriter {
	if n == nil {
		return nil
	}
	return timedGeneratedWriter{t, n}
}

type timedGeneratedReader struct {
	t timer
	n data.GeneratedReader
}

func (w timedGeneratedReader) GetGenerated(ctx context.Context, k data.GenKey) (o data.GeneratedOutput, err error) {
	defer w.t.start(ctx, "GeneratedReader", "GetGenerated", &err)()
	return w.n.GetGenerated(ctx, k)
}

func (w timedGeneratedReader) GetCurrent(
	ctx context.Context, task string, v data.VersionRef, target string,
) (o data.GeneratedOutput, err error) {
	defer w.t.start(ctx, "GeneratedReader", "GetCurrent", &err)()
	return w.n.GetCurrent(ctx, task, v, target)
}

func (w timedGeneratedReader) GetGrade(ctx context.Context, k data.GenKey) (g data.Grade, err error) {
	defer w.t.start(ctx, "GeneratedReader", "GetGrade", &err)()
	return w.n.GetGrade(ctx, k)
}

type timedGeneratedWriter struct {
	t timer
	n data.GeneratedWriter
}

func (w timedGeneratedWriter) PutGenerated(ctx context.Context, o data.GeneratedOutput) (err error) {
	defer w.t.start(ctx, "GeneratedWriter", "PutGenerated", &err)()
	return w.n.PutGenerated(ctx, o)
}

func (w timedGeneratedWriter) PutRevised(ctx context.Context, o data.GeneratedOutput) (err error) {
	defer w.t.start(ctx, "GeneratedWriter", "PutRevised", &err)()
	return w.n.PutRevised(ctx, o)
}

func (w timedGeneratedWriter) SetCurrent(ctx context.Context, o data.GeneratedOutput) (err error) {
	defer w.t.start(ctx, "GeneratedWriter", "SetCurrent", &err)()
	return w.n.SetCurrent(ctx, o)
}

func (w timedGeneratedWriter) PutGrade(ctx context.Context, g data.Grade) (err error) {
	defer w.t.start(ctx, "GeneratedWriter", "PutGrade", &err)()
	return w.n.PutGrade(ctx, g)
}

func (w timedGeneratedWriter) SaveBatch(ctx context.Context, b data.GenBatch) (err error) {
	defer w.t.start(ctx, "GeneratedWriter", "SaveBatch", &err)()
	return w.n.SaveBatch(ctx, b)
}

func (w timedGeneratedWriter) PendingBatches(ctx context.Context) (b []data.GenBatch, err error) {
	defer w.t.start(ctx, "GeneratedWriter", "PendingBatches", &err)()
	return w.n.PendingBatches(ctx)
}
