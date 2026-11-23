package generate

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

//go:embed prompts/*.md schemas/*.json
var files embed.FS

// Default models. The grader must differ from the generator; both are
// recorded with every stored output and grade.
const (
	DefaultModel       = "claude-opus-5-5"
	DefaultGraderModel = "claude-sonnet-5-5"
)

// Prompt is one task's prompt files: the shared instructions, the task text
// and the output schema, with a version that changes with any of them.
type Prompt struct {
	Instructions string
	Task         string
	Schema       []byte
	Version      string
}

// taskFiles maps a task to its prompt and schema files.
func taskFiles(task string) (prompt, schema string, ok bool) {
	switch task {
	case "bill-summary", "version-changes":
		return "prompts/" + task + ".md", "schemas/summary.json", true
	case "change-explanation", "bill-explanation":
		return "prompts/" + task + ".md", "schemas/explanation.json", true
	case "grade":
		return "prompts/grade.md", "schemas/grade.json", true
	case "crs":
		return "prompts/crs.md", "schemas/crs.json", true
	}
	return "", "", false
}

// LoadPrompt reads the embedded prompt files of a task. The version is "v1-"
// plus the first 8 hex characters of the hash of the instructions, task text
// and schema, so any prompt edit creates new idempotency keys.
func LoadPrompt(task string) (Prompt, error) {
	pf, sf, ok := taskFiles(task)
	if !ok {
		return Prompt{}, fmt.Errorf("prompt for task %q: %w", task, data.ErrInvalid)
	}
	read := func(name string) ([]byte, error) {
		b, err := files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", name, err)
		}
		return b, nil
	}
	common, err := read("prompts/common.md")
	if err != nil {
		return Prompt{}, err
	}
	text, err := read(pf)
	if err != nil {
		return Prompt{}, err
	}
	schema, err := read(sf)
	if err != nil {
		return Prompt{}, err
	}
	h := sha256.New()
	for _, b := range [][]byte{common, text, schema} {
		h.Write(b)
		h.Write([]byte{0})
	}
	return Prompt{
		Instructions: string(common), Task: string(text), Schema: schema,
		Version: "v1-" + hex.EncodeToString(h.Sum(nil))[:8],
	}, nil
}
