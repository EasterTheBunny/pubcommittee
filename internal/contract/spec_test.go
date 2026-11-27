package contract_test

import (
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/easterthebunny/pubcommittee/internal/contract"
)

var snakeCase = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// textProps are property names that hold prose; a schema with one must either
// carry a content class (x-carries-text) or inherit it (x-inherits-meta).
func textProps() []string {
	return []string{
		"text", "title", "headline", "what_changes", "why_it_matters", "reason",
		"notice", "rules", "heading", "exclusion_reason",
	}
}

// flatten returns every property and required name of s, following allOf.
func flatten(s *openapi3.Schema) (props map[string]*openapi3.SchemaRef, required map[string]bool) {
	props = map[string]*openapi3.SchemaRef{}
	required = map[string]bool{}
	var walk func(*openapi3.Schema)
	walk = func(s *openapi3.Schema) {
		maps.Copy(props, s.Properties)
		for _, r := range s.Required {
			required[r] = true
		}
		for _, sub := range s.AllOf {
			walk(sub.Value)
		}
	}
	walk(s)
	return props, required
}

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := contract.Spec("http://localhost")
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func hasExt(s *openapi3.Schema, key string) bool {
	v, ok := s.Extensions[key]
	return ok && v == true
}

// carriesText reports x-carries-text on s or any allOf member.
func carriesText(s *openapi3.Schema) bool {
	if hasExt(s, "x-carries-text") {
		return true
	}
	return slices.ContainsFunc(s.AllOf, func(sub *openapi3.SchemaRef) bool { return carriesText(sub.Value) })
}

func TestEveryTextSchemaDeclaresContentClass(t *testing.T) {
	t.Parallel()
	doc := loadSpec(t)
	for name, ref := range doc.Components.Schemas {
		checkTextSchema(t, name, ref.Value)
	}
}

func checkTextSchema(t *testing.T, name string, s *openapi3.Schema) {
	t.Helper()
	props, required := flatten(s)
	carries := carriesText(s)
	inherits := hasExt(s, "x-inherits-meta") || s.Extensions["x-exempt-content-class"] != nil
	hasText := slices.ContainsFunc(textProps(), func(p string) bool { _, ok := props[p]; return ok })
	if hasText && !carries && !inherits {
		t.Errorf("schema %s has a text property but neither x-carries-text nor x-inherits-meta", name)
	}
	if !carries {
		return
	}
	if !required["content_class"] {
		t.Errorf("schema %s carries text but does not require content_class", name)
	}
	if slices.Contains([]string{"ClassedText", "AsAmendedPreview", "Statement"}, name) {
		return
	}
	for _, f := range []string{"version", "as_of", "provenance"} {
		if !required[f] {
			t.Errorf("record schema %s does not require %s", name, f)
		}
	}
}

func TestSummaryClassesExcludeUser(t *testing.T) {
	t.Parallel()
	doc := loadSpec(t)
	for _, v := range doc.Components.Schemas["TrustedContentClass"].Value.Enum {
		if v == "user" {
			t.Fatal("TrustedContentClass allows user text")
		}
	}
	meta := doc.Components.Schemas["Meta"].Value.Properties["content_class"]
	if meta.Ref != "#/components/schemas/TrustedContentClass" {
		t.Fatalf("Meta.content_class refers to %q, want TrustedContentClass", meta.Ref)
	}
}

func TestPathsAndOperations(t *testing.T) {
	t.Parallel()
	doc := loadSpec(t)
	for path, item := range doc.Paths.Map() {
		checkPath(t, path, item)
	}
}

func checkPath(t *testing.T, path string, item *openapi3.PathItem) {
	t.Helper()
	if !strings.HasPrefix(path, "/v1") || strings.Contains(strings.ToLower(path), "author") {
		t.Errorf("path %s is outside /v1 or addresses authors", path)
	}
	op := item.Operations()[http.MethodGet]
	if len(item.Operations()) != 1 || op == nil {
		t.Errorf("path %s must have exactly one GET operation", path)
		return
	}
	for _, p := range slices.Concat(item.Parameters, op.Parameters) {
		checkParam(t, path, p.Value)
	}
	checkResponses(t, path, op)
	if _, has := op.Extensions["x-cache"]; !has {
		t.Errorf("GET %s lacks x-cache", path)
	}
}

func checkResponses(t *testing.T, path string, op *openapi3.Operation) {
	t.Helper()
	for _, code := range []string{"200", "304", "429", "503"} {
		if op.Responses.Value(code) == nil {
			t.Errorf("GET %s lacks a %s response", path, code)
		}
	}
	if ok := op.Responses.Value("200"); ok != nil {
		if _, has := ok.Value.Headers["ETag"]; !has {
			t.Errorf("GET %s 200 does not document ETag", path)
		}
	}
}

func checkParam(t *testing.T, path string, p *openapi3.Parameter) {
	t.Helper()
	if strings.Contains(strings.ToLower(p.Name), "author") {
		t.Errorf("GET %s filters by author via %s", path, p.Name)
	}
}

func TestSnakeCaseProperties(t *testing.T) {
	t.Parallel()
	doc := loadSpec(t)
	for name, ref := range doc.Components.Schemas {
		props, _ := flatten(ref.Value)
		for p := range props {
			if !snakeCase.MatchString(p) {
				t.Errorf("schema %s property %q is not snake_case", name, p)
			}
		}
	}
}

func TestProblemTypes(t *testing.T) {
	t.Parallel()
	doc := loadSpec(t)
	enum := doc.Components.Schemas["Problem"].Value.Properties["type"].Value.Enum
	want := []string{"not-found", "conflict", "invalid", "rate-limited", "unavailable"}
	if len(enum) != len(want) {
		t.Fatalf("problem types %v, want %v", enum, want)
	}
	for _, w := range want {
		if !slices.Contains(enum, any("/problems/"+w)) {
			t.Errorf("problem type /problems/%s missing", w)
		}
	}
}

func TestEveryM1RoutePresent(t *testing.T) {
	t.Parallel()
	doc := loadSpec(t)
	for _, p := range []string{
		"/v1", "/v1/openapi.yaml", "/v1/bills", "/v1/bills/{bill}", "/v1/bills/{bill}/v/{version}",
		"/v1/bills/{bill}/v/{version}/s/{section}", "/v1/bills/{bill}/v/{version}/s/{section}/l/{range}",
		"/v1/bills/{bill}/s/{section}", "/v1/bills/{bill}/s/{section}/l/{range}",
		"/v1/bills/{bill}/v/{version}/explanations", "/v1/bills/{bill}/v/{version}/summary",
		"/v1/bills/{bill}/v/{version}/changes", "/v1/bills/{bill}/v/{version}/citations",
		"/v1/bills/{bill}/v/{version}/citations/unresolved", "/v1/bills/{bill}/v/{version}/compare/{other}",
		"/v1/selection-policy", "/v1/selection-policy/{version}", "/v1/moderation-log",
		"/v1/feed", "/v1/exports", "/v1/exports/{export}",
	} {
		if doc.Paths.Value(p) == nil {
			t.Errorf("route %s missing from the spec", p)
		}
	}
	for p := range doc.Paths.Map() {
		for _, banned := range []string{"/auth", "/me", "/signals", "/threads", "/suggestions", "/briefs", "/snapshots", "/steward"} {
			if strings.HasPrefix(p, "/v1"+banned) || strings.HasPrefix(p, banned) {
				t.Errorf("route %s belongs to a later milestone or another service", p)
			}
		}
	}
}
