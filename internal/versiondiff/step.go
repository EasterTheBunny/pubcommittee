package versiondiff

import (
	"context"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Step is the comparison step that runs after a new version is stored: it
// computes and writes the comparison of every consecutive pair of stored
// versions and of every older version with the latest, so an old link always
// has a direct answer. Writes are write-once,
// so repeating the step is a no-op for pairs already stored. It returns the
// number of comparisons computed. Nothing runs it automatically in the local
// phase; a command or job calls it over stored data.
func Step(
	ctx context.Context, text data.BillText, store data.ComparisonWriter, bill data.BillRef, parserVersion string,
) (int, error) {
	vs, err := text.ListVersions(ctx, bill)
	if err != nil {
		return 0, fmt.Errorf("compare %s: list versions: %w", bill, err)
	}
	pairs := Pairs(len(vs))
	versions := map[int]data.OfficialVersion{}
	get := func(i int) (data.OfficialVersion, error) {
		if v, ok := versions[i]; ok {
			return v, nil
		}
		v, err := text.GetVersion(ctx, vs[i].Ref)
		if err != nil {
			return data.OfficialVersion{}, fmt.Errorf("compare %s: get version %s: %w", bill, vs[i].Ref, err)
		}
		versions[i] = v
		return v, nil
	}
	for _, p := range pairs {
		from, err := get(p[0])
		if err != nil {
			return 0, err
		}
		to, err := get(p[1])
		if err != nil {
			return 0, err
		}
		if err := store.PutComparison(ctx, Compare(from, to, parserVersion)); err != nil {
			return 0, fmt.Errorf("compare %s: store: %w", bill, err)
		}
	}
	return len(pairs), nil
}

// Pairs lists the index pairs (older, newer) the step compares for n
// versions, oldest first: every consecutive pair, then every older version
// with the latest.
func Pairs(n int) [][2]int {
	var out [][2]int
	for i := 0; i+1 < n; i++ {
		out = append(out, [2]int{i, i + 1})
	}
	for i := 0; i+2 < n; i++ {
		out = append(out, [2]int{i, n - 1})
	}
	return out
}
