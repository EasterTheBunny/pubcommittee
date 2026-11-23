package generate

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

func TestShortTitleOnly(t *testing.T) {
	t.Parallel()
	line := func(text string) data.Line { return data.Line{Text: text} }
	cases := []struct {
		name string
		sec  data.Section
		want bool
	}{
		{"short title", data.Section{Lines: []data.Line{line("This Act may be cited as the “Example Act”.")}}, true},
		{"short title with heading line", data.Section{Lines: []data.Line{
			line("(a) Short title This Act may be cited as the “Example Act of 2025”."),
		}}, true},
		{"short title and table of contents", data.Section{Lines: []data.Line{
			line("(a) Short title This Act may be cited as the “Example Act”."),
			line("(b) Table of contents The table of contents for this Act is as follows:"),
		}}, false},
		{"other provision", data.Section{Lines: []data.Line{line("The Secretary shall publish a report.")}}, false},
		{"no lines", data.Section{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := shortTitleOnly(c.sec); got != c.want {
				t.Fatalf("shortTitleOnly = %v, want %v", got, c.want)
			}
		})
	}
}
