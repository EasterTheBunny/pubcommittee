package fixturesrc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// lawDir is the directory of the example US Code sections, one directory per
// release point: uscode/<id>/release.json and uscode/<id>/t<title>-s<section>.xml.
const lawDir = "uscode"

var _ data.LawSource = (*Source)(nil)

type releaseFile struct {
	ID   string `json:"id"`
	Date string `json:"date"`
}

// ListReleasePoints lists the release points the fixtures hold, newest first.
// The release date is not in the section files; release.json carries it.
func (s *Source) ListReleasePoints(ctx context.Context) ([]data.ReleasePoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fixturesrc: list release points: %w", err)
	}
	entries, err := fs.ReadDir(s.fsys, lawDir)
	if err != nil {
		return nil, fmt.Errorf("fixturesrc: read %s: %w: %w", lawDir, data.ErrUnavailable, err)
	}
	var out []data.ReleasePoint
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := path.Join(lawDir, e.Name(), "release.json")
		b, err := fs.ReadFile(s.fsys, p)
		if err != nil {
			return nil, fmt.Errorf("fixturesrc: read %s: %w: %w", p, data.ErrUnavailable, err)
		}
		var rf releaseFile
		if err := json.Unmarshal(b, &rf); err != nil || rf.ID != e.Name() {
			return nil, fmt.Errorf("fixturesrc: %s: id must be %q: %w", p, e.Name(), data.ErrInvalid)
		}
		d, err := time.Parse(time.DateOnly, rf.Date)
		if err != nil {
			return nil, fmt.Errorf("fixturesrc: %s: date: %w: %w", p, data.ErrInvalid, err)
		}
		out = append(out, data.ReleasePoint{ID: rf.ID, Date: d.UTC()})
	}
	slices.SortFunc(out, func(a, b data.ReleasePoint) int { return strings.Compare(b.ID, a.ID) })
	return out, nil
}

// titleFiles lists the section files of one title at a release point.
func (s *Source) titleFiles(rp data.ReleasePoint, title int) ([]string, error) {
	files, err := fs.Glob(s.fsys, path.Join(lawDir, rp.ID, "t"+strconv.Itoa(title)+"-s*.xml"))
	if err != nil {
		return nil, fmt.Errorf("fixturesrc: glob title %d: %w: %w", title, data.ErrInvalid, err)
	}
	return files, nil
}

// LawTitles lists the titles that hold example sections at a release point,
// ascending.
func (s *Source) LawTitles(ctx context.Context, rp data.ReleasePoint) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fixturesrc: law titles: %w", err)
	}
	files, err := fs.Glob(s.fsys, path.Join(lawDir, rp.ID, "t*-s*.xml"))
	if err != nil {
		return nil, fmt.Errorf("fixturesrc: glob titles: %w: %w", data.ErrInvalid, err)
	}
	var out []int
	for _, f := range files {
		var title int
		if _, err := fmt.Sscanf(path.Base(f), "t%d-", &title); err != nil {
			return nil, fmt.Errorf("fixturesrc: file name %s: %w: %w", f, data.ErrInvalid, err)
		}
		if !slices.Contains(out, title) {
			out = append(out, title)
		}
	}
	slices.Sort(out)
	return out, nil
}

// FetchTitle returns the sections the fixtures hold for a title, each cut
// byte for byte from the official file, wrapped in a uscDoc root so the USLM
// parser reads them; ErrNotFound when the fixtures hold none.
func (s *Source) FetchTitle(ctx context.Context, rp data.ReleasePoint, title int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fixturesrc: fetch title: %w", err)
	}
	files, err := s.titleFiles(rp, title)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("fixturesrc: title %d of %s: %w", title, rp.ID, data.ErrNotFound)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<uscDoc identifier="/us/usc/t%d"><main>`, title)
	for _, f := range files {
		frag, err := fs.ReadFile(s.fsys, f)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("fixturesrc: %s: %w", f, data.ErrNotFound)
		}
		if err != nil {
			return nil, fmt.Errorf("fixturesrc: read %s: %w: %w", f, data.ErrUnavailable, err)
		}
		b.Write(frag)
	}
	b.WriteString(`</main></uscDoc>`)
	return []byte(b.String()), nil
}
