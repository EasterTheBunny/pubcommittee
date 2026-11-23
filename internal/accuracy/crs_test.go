package accuracy_test

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/accuracy"
	"github.com/easterthebunny/pubcommittee/internal/adapters/fakegen"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

const crsFixture = "../../fixtures/bills/us-119-hr-4313/status.xml"

func TestParseCRSSummariesFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(crsFixture)
	if err != nil {
		t.Fatal(err)
	}
	got, err := accuracy.ParseCRSSummaries(raw)
	if err != nil {
		t.Fatalf("ParseCRSSummaries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d summaries, want 1", len(got))
	}
	if got[0].VersionCode != "00" {
		t.Errorf("VersionCode = %q, want %q", got[0].VersionCode, "00")
	}
	if !strings.HasPrefix(got[0].Text, "Hospital Inpatient Services Modernization Act") {
		t.Errorf("Text starts %q", got[0].Text[:min(len(got[0].Text), 60)])
	}
	if strings.Contains(got[0].Text, "<") {
		t.Errorf("Text contains markup: %q", got[0].Text)
	}
	if !strings.Contains(got[0].Text, "Centers for Medicare & Medicaid Services") {
		t.Errorf("entity &amp; not decoded: %q", got[0].Text)
	}
}

func TestParseCRSSummariesEntitiesAndParagraphs(t *testing.T) {
	t.Parallel()
	const doc = `<?xml version="1.0" encoding="utf-8"?>
<billStatus><bill><summaries><summary><versionCode>00</versionCode><text><![CDATA[<p>A &amp; B &lt;x&gt; &quot;q&quot; &#39;s&#39;&nbsp;z</p><p>Second</p>]]></text></summary></summaries></bill></billStatus>`
	got, err := accuracy.ParseCRSSummaries([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := []accuracy.CRSSummary{{VersionCode: "00", Text: "A & B <x> \"q\" 's' z\nSecond"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParseCRSSummariesErrors(t *testing.T) {
	t.Parallel()
	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		_, err := accuracy.ParseCRSSummaries([]byte(`<billStatus><summaries><summary>`))
		if !errors.Is(err, data.ErrInvalid) {
			t.Fatalf("err = %v, want ErrInvalid", err)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		got, err := accuracy.ParseCRSSummaries([]byte(`<billStatus><bill><summaries></summaries></bill></billStatus>`))
		if err != nil || got != nil {
			t.Fatalf("got %#v, %v; want nil, nil", got, err)
		}
	})
}

const (
	crsTestReference = "The bill extends the hospital program"
	crsTestGenerated = "Summary of the bill text."
)

func TestCRSRequestInput(t *testing.T) {
	t.Parallel()
	req, err := accuracy.CRSRequest("crs-1", crsTestGenerated, crsTestReference, "grader-model")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(req.Input, "<request>") || !strings.HasSuffix(req.Input, "</request>") {
		t.Errorf("Input not wrapped in <request>: %q", req.Input)
	}
	if !strings.Contains(req.Input, crsTestReference) || !strings.Contains(req.Input, crsTestGenerated) {
		t.Errorf("Input lacks reference or generated text: %q", req.Input)
	}
}

func TestCRSRequestFields(t *testing.T) {
	t.Parallel()
	req, err := accuracy.CRSRequest("crs-1", crsTestGenerated, crsTestReference, "grader-model")
	if err != nil {
		t.Fatal(err)
	}
	if req.Task != generate.TaskCRS || req.Model != "grader-model" || req.Effort != "medium" {
		t.Errorf("Task/Model/Effort = %q/%q/%q", req.Task, req.Model, req.Effort)
	}
	if !strings.HasPrefix(req.PromptVersion, "v1-") || req.Instructions == "" || len(req.Schema) == 0 {
		t.Errorf("prompt not loaded: version %q", req.PromptVersion)
	}
}

func TestCRSRequestEmptyModel(t *testing.T) {
	t.Parallel()
	_, err := accuracy.CRSRequest("crs-1", "g", "r", "")
	if !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestParseCRS(t *testing.T) {
	t.Parallel()
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		got, err := accuracy.ParseCRS(`{"points":[{"point":"a","verdict":"covers"},{"point":"b","verdict":"misses"},{"point":"c","verdict":"contradicts"}]}`)
		if err != nil {
			t.Fatal(err)
		}
		want := []accuracy.CRSPoint{{Point: "a", Verdict: "covers"}, {Point: "b", Verdict: "misses"}, {Point: "c", Verdict: "contradicts"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	})
	bad := map[string]string{
		"unknown verdict": `{"points":[{"point":"a","verdict":"maybe"}]}`,
		"unknown field":   `{"points":[{"point":"a","verdict":"covers","score":1}]}`,
		"unknown top":     `{"points":[],"extra":true}`,
		"no points":       `{}`,
		"trailing data":   `{"points":[]} {"points":[]}`,
		"not json":        `points`,
	}
	for name, text := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := accuracy.ParseCRS(text)
			if !errors.Is(err, data.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestCoverage(t *testing.T) {
	t.Parallel()
	points := []accuracy.CRSPoint{
		{Point: "a", Verdict: "covers"},
		{Point: "b", Verdict: "covers"},
		{Point: "c", Verdict: "misses"},
		{Point: "d", Verdict: "contradicts"},
	}
	total, covered, contradicts := accuracy.Coverage(points)
	if total != 4 || covered != 2 || contradicts != 1 {
		t.Errorf("Coverage = %d, %d, %d; want 4, 2, 1", total, covered, contradicts)
	}
	total, covered, contradicts = accuracy.Coverage(nil)
	if total != 0 || covered != 0 || contradicts != 0 {
		t.Errorf("Coverage(nil) = %d, %d, %d", total, covered, contradicts)
	}
}

func TestCRSFakegenCovers(t *testing.T) {
	t.Parallel()
	const reference = "The bill extends the Acute Hospital Care at Home Program through fiscal year 2030. The bill requires a study of patient outcomes."
	req, err := accuracy.CRSRequest("crs-fake", reference, reference, "fakegen")
	if err != nil {
		t.Fatal(err)
	}
	res, err := fakegen.New(fakegen.Config{}).Generate(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "ok" {
		t.Fatalf("Outcome = %q, want ok", res.Outcome)
	}
	points, err := accuracy.ParseCRS(res.Text)
	if err != nil {
		t.Fatalf("ParseCRS: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2", len(points))
	}
	for _, p := range points {
		if p.Verdict != "covers" {
			t.Errorf("point %q verdict %q, want covers", p.Point, p.Verdict)
		}
	}
}
