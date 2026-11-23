// Package claude implements data.Generator with the Anthropic Go SDK: Message
// Batches for the two-phase job, one synchronous call for a steward-requested
// regeneration. The synchronous call enables the server-side fallbacks beta in
// its "default" form; the Batches API rejects it, so batches run without.
// Every request carries a JSON-schema output format (structured outputs), an
// explicit effort, and a cached prefix of instructions plus the bill's full
// text. Callers import only this package, never the SDK.
//
// New builds the first-party client for local live runs (stage L3, an
// operator-supplied key); NewGoogleCloud (gcp builds) uses Claude Models on
// Google Cloud with Application Default Credentials and no key.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// DefaultModel is the generating model of the architecture.
const DefaultModel = "claude-opus-5-5"

// maxRetries is the SDK retry count for 408, 409, 429 and 5xx answers.
const maxRetries = 2

// defaultMaxTokens bounds thinking plus the JSON answer of one request.
const defaultMaxTokens = 16000

// fallbackBeta is the beta header of the server-side fallbacks parameter; its
// "default" form needs this value (architecture § AI-generated content).
const fallbackBeta = anthropic.AnthropicBetaServerSideFallback2026_07_01

// iterFallback is the usage.iterations type that shows a fallback model ran.
const iterFallback = "fallback_message"

// Outcomes of data.GenResult.Outcome.
const (
	outcomeOK        = "ok"
	outcomeRefusal   = "refusal"
	outcomeTruncated = "truncated"
	outcomeInvalid   = "invalid"
	outcomeError     = "error"
)

// Config configures the first-party client. APIKey is read by the caller from
// the operator's environment and is never stored. BaseURL is for tests.
type Config struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int64
}

// Gen implements data.Generator over the Messages and Message Batches APIs.
type Gen struct {
	messages  anthropic.MessageService
	beta      anthropic.BetaMessageService
	model     string
	maxTokens int64
}

var _ data.Generator = (*Gen)(nil)

// New builds the first-party client. It fails fast on a missing key and does
// not touch the network.
func New(cfg Config) (*Gen, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("claude: API key is required: %w", data.ErrInvalid)
	}
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey), option.WithMaxRetries(maxRetries)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	c := anthropic.NewClient(opts...)
	return newGen(c.Messages, c.Beta.Messages, cfg.Model, cfg.MaxTokens), nil
}

func newGen(m anthropic.MessageService, b anthropic.BetaMessageService, model string, maxTokens int64) *Gen {
	if model == "" {
		model = DefaultModel
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	return &Gen{messages: m, beta: b, model: model, maxTokens: maxTokens}
}

// Submit starts one Message Batch with a request per GenRequest.
func (g *Gen) Submit(ctx context.Context, reqs []data.GenRequest) (string, error) {
	if len(reqs) == 0 {
		return "", fmt.Errorf("submit: no requests: %w", data.ErrInvalid)
	}
	items := make([]anthropic.MessageBatchNewParamsRequest, 0, len(reqs))
	seen := make(map[string]bool, len(reqs))
	for _, r := range reqs {
		if err := validate(r); err != nil {
			return "", fmt.Errorf("submit: %w", err)
		}
		if seen[r.ID] {
			return "", fmt.Errorf("submit: duplicate request ID %q: %w", r.ID, data.ErrInvalid)
		}
		seen[r.ID] = true
		p, err := g.build(r)
		if err != nil {
			return "", fmt.Errorf("submit: %w", err)
		}
		items = append(items, anthropic.MessageBatchNewParamsRequest{
			CustomID: r.ID,
			Params: anthropic.MessageBatchNewParamsRequestParams{
				MaxTokens: p.MaxTokens, Model: p.Model, System: p.System, Messages: p.Messages,
				OutputConfig: p.OutputConfig, Thinking: p.Thinking,
			},
		})
	}
	b, err := g.messages.Batches.New(ctx, anthropic.MessageBatchNewParams{Requests: items})
	if err != nil {
		return "", fmt.Errorf("submit batch: %w", classify(err))
	}
	return b.ID, nil
}

// Collect reports done=false while the batch is processing, and the results
// of every request once it has ended.
func (g *Gen) Collect(ctx context.Context, batchID string) ([]data.GenResult, bool, error) {
	b, err := g.messages.Batches.Get(ctx, batchID, anthropic.MessageBatchGetParams{})
	if err != nil {
		return nil, false, fmt.Errorf("collect batch %s: %w", batchID, classify(err))
	}
	if b.ProcessingStatus != anthropic.MessageBatchProcessingStatusEnded {
		return nil, false, nil
	}
	stream := g.messages.Batches.ResultsStreaming(ctx, batchID, anthropic.MessageBatchResultsParams{})
	defer func() { _ = stream.Close() }()
	var out []data.GenResult
	for stream.Next() {
		out = append(out, g.fromBatch(stream.Current()))
	}
	if err := stream.Err(); err != nil {
		return nil, false, fmt.Errorf("collect batch %s: results: %w", batchID, classify(err))
	}
	return out, true, nil
}

// Generate runs one request synchronously on the beta Messages endpoint with
// the server-side fallbacks parameter set to "default": a policy decline is
// re-run on the model Anthropic recommends, inside the same call.
func (g *Gen) Generate(ctx context.Context, req data.GenRequest) (data.GenResult, error) {
	if err := validate(req); err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", err)
	}
	p, err := g.build(req)
	if err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", err)
	}
	msg, err := g.beta.New(ctx, betaParams(p, req))
	if err != nil {
		return data.GenResult{}, fmt.Errorf("generate: %w", classify(err))
	}
	return fromBetaMessage(req.ID, p.Model, msg), nil
}

// betaParams maps the built request to the beta endpoint, adding the
// fallbacks beta header and the "default" fallbacks field. Cache breakpoints,
// the output format and the effort carry over unchanged.
func betaParams(p anthropic.MessageNewParams, r data.GenRequest) anthropic.BetaMessageNewParams {
	bp := anthropic.BetaMessageNewParams{
		MaxTokens: p.MaxTokens,
		Model:     p.Model,
		Betas:     []anthropic.AnthropicBeta{fallbackBeta},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
	}
	for _, t := range p.System {
		bp.System = append(bp.System, anthropic.BetaTextBlockParam{Text: t.Text})
	}
	var blocks []anthropic.BetaContentBlockParamUnion
	if r.Source != "" {
		src := anthropic.NewBetaTextBlock(r.Source)
		src.OfText.CacheControl = anthropic.NewBetaCacheControlEphemeralParam()
		blocks = append(blocks, src)
	}
	blocks = append(blocks, anthropic.NewBetaTextBlock(r.Input))
	bp.Messages = []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(blocks...)}
	if p.OutputConfig.Format.Schema != nil {
		bp.OutputConfig.Format = anthropic.BetaJSONOutputFormatParam{Schema: p.OutputConfig.Format.Schema}
	}
	if p.OutputConfig.Effort != "" {
		bp.OutputConfig.Effort = anthropic.BetaOutputConfigEffort(p.OutputConfig.Effort)
	}
	return bp
}

func validate(r data.GenRequest) error {
	if r.ID == "" || r.Task == "" {
		return fmt.Errorf("request ID %q task %q: %w", r.ID, r.Task, data.ErrInvalid)
	}
	return nil
}

// build maps a request to Messages API parameters: the shared instructions
// go in system; the bill source is the first user block and carries the cache
// breakpoint (so system and source are cached together); the volatile request
// is the second block. Thinking is left at the model's adaptive default.
func (g *Gen) build(r data.GenRequest) (anthropic.MessageNewParams, error) {
	model := g.model
	if r.Model != "" {
		model = r.Model
	}
	p := anthropic.MessageNewParams{
		MaxTokens: g.maxTokens,
		Model:     model,
	}
	if r.Instructions != "" {
		p.System = []anthropic.TextBlockParam{{Text: r.Instructions}}
	}
	var blocks []anthropic.ContentBlockParamUnion
	if r.Source != "" {
		src := anthropic.NewTextBlock(r.Source)
		src.OfText.CacheControl = anthropic.NewCacheControlEphemeralParam()
		blocks = append(blocks, src)
	}
	blocks = append(blocks, anthropic.NewTextBlock(r.Input))
	p.Messages = []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)}
	if len(r.Schema) > 0 {
		var schema map[string]any
		if err := json.Unmarshal(r.Schema, &schema); err != nil {
			return p, fmt.Errorf("request %s: schema: %w: %w", r.ID, data.ErrInvalid, err)
		}
		p.OutputConfig.Format = anthropic.JSONOutputFormatParam{Schema: schema}
	}
	if r.Effort != "" {
		p.OutputConfig.Effort = anthropic.OutputConfigEffort(r.Effort)
	}
	return p, nil
}

func (g *Gen) fromBatch(r anthropic.MessageBatchIndividualResponse) data.GenResult {
	if r.Result.Type != "succeeded" {
		return data.GenResult{ID: r.CustomID, Outcome: outcomeError, Stop: r.Result.Type, Model: g.model}
	}
	return fromMessage(r.CustomID, &r.Result.Message)
}

// fromMessage maps a response to a result. The stop reason is checked before
// any content is read: a refusal or max_tokens answer is never parsed.
func fromMessage(id string, m *anthropic.Message) data.GenResult {
	res := data.GenResult{
		ID: id, Model: m.Model, Stop: string(m.StopReason),
		TokensIn:  int(m.Usage.InputTokens + m.Usage.CacheReadInputTokens + m.Usage.CacheCreationInputTokens),
		TokensOut: int(m.Usage.OutputTokens), CachedIn: int(m.Usage.CacheReadInputTokens),
	}
	if outcome := stopOutcome(string(m.StopReason)); outcome != outcomeOK {
		res.Outcome = outcome
		return res
	}
	var sb strings.Builder
	for _, b := range m.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	res.Text = sb.String()
	res.Outcome = outcomeOK
	if res.Text == "" {
		res.Outcome = outcomeInvalid
	}
	return res
}

// fromBetaMessage maps a beta response like fromMessage. Message.Model names
// the model that produced the message; when a fallback model ran (a
// fallback_message iteration or a fallback block), FallbackFrom names the
// requested model. A refusal or max_tokens stop the fallback did not resolve
// keeps its outcome and no text.
func fromBetaMessage(id, requested string, m *anthropic.BetaMessage) data.GenResult {
	res := data.GenResult{
		ID: id, Model: m.Model, Stop: string(m.StopReason),
		TokensIn:  int(m.Usage.InputTokens + m.Usage.CacheReadInputTokens + m.Usage.CacheCreationInputTokens),
		TokensOut: int(m.Usage.OutputTokens), CachedIn: int(m.Usage.CacheReadInputTokens),
	}
	for _, it := range m.Usage.Iterations {
		if it.Type == iterFallback {
			res.FallbackFrom = requested
		}
	}
	for _, b := range m.Content {
		if b.Type == "fallback" {
			res.FallbackFrom = requested
		}
	}
	if outcome := stopOutcome(string(m.StopReason)); outcome != outcomeOK {
		res.Outcome = outcome
		return res
	}
	var sb strings.Builder
	for _, b := range m.Content {
		if t, ok := b.AsAny().(anthropic.BetaTextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	res.Text = sb.String()
	res.Outcome = outcomeOK
	if res.Text == "" {
		res.Outcome = outcomeInvalid
	}
	return res
}

// stopOutcome maps a stop reason: only a normal end of turn may be read.
func stopOutcome(s string) string {
	switch s {
	case string(anthropic.StopReasonRefusal):
		return outcomeRefusal
	case string(anthropic.StopReasonMaxTokens):
		return outcomeTruncated
	case string(anthropic.StopReasonEndTurn):
		return outcomeOK
	default:
		return outcomeInvalid
	}
}

// classify maps SDK errors to the typed data errors so vendor errors never
// cross the port.
func classify(err error) error {
	if api, ok := errors.AsType[*anthropic.Error](err); ok {
		switch {
		case api.StatusCode == http.StatusTooManyRequests:
			return fmt.Errorf("%w: status %d", data.ErrRateLimited, api.StatusCode)
		case api.StatusCode == http.StatusNotFound:
			return fmt.Errorf("%w: status %d", data.ErrNotFound, api.StatusCode)
		case api.StatusCode >= http.StatusInternalServerError:
			return fmt.Errorf("%w: status %d", data.ErrUnavailable, api.StatusCode)
		default:
			return fmt.Errorf("%w: status %d", data.ErrInvalid, api.StatusCode)
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %w", data.ErrUnavailable, err)
}
