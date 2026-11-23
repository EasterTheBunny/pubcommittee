// Command extract-uscode-fixtures cuts the cited sections out of downloaded
// US Code title files (kept in the git-ignored .local-data/) and writes them
// byte for byte under fixtures/uscode/<release>/, with a table of source
// hashes for fixtures/README.md.
//
//	go run ./scripts/extract-uscode-fixtures -zips .local-data/uscode/119-142 \
//	    -release 119-142 -out fixtures/uscode 42/1395cc–7 15/717b ...
//
// Section numbers are given as in the source (en dash included).
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	dirMode  = 0o750
	fileMode = 0o600
)

var errInput = errors.New("bad input")

func main() {
	zips := flag.String("zips", "", "directory of xml_usc<TT>@<release>.zip files")
	release := flag.String("release", "", "release point id, for example 119-142")
	out := flag.String("out", "fixtures/uscode", "output directory")
	flag.Parse()
	if err := run(*zips, *release, *out, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "extract-uscode-fixtures:", err)
		os.Exit(1)
	}
}

func run(zips, release, out string, specs []string) error {
	titles := map[int][]string{}
	for _, s := range specs {
		t, sec, ok := strings.Cut(s, "/")
		n, err := strconv.Atoi(t)
		if !ok || err != nil {
			return fmt.Errorf("spec %q, want <title>/<section>: %w", s, errInput)
		}
		titles[n] = append(titles[n], sec)
	}
	dir := filepath.Join(out, release)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	for n, secs := range titles {
		file := fmt.Sprintf("xml_usc%02d@%s.zip", n, release)
		xmlBytes, err := readTitle(filepath.Join(zips, file), fmt.Sprintf("usc%02d.xml", n))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(xmlBytes)
		_, _ = fmt.Fprintf(os.Stdout, "title %d file sha256 %s\n", n, hex.EncodeToString(sum[:]))
		for _, sec := range secs {
			if err := writeSection(xmlBytes, dir, n, sec); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeSection cuts one section out of a title file and writes it.
func writeSection(xmlBytes []byte, dir string, title int, sec string) error {
	marker := fmt.Sprintf(`identifier="/us/usc/t%d/s%s"`, title, sec)
	before, _, ok := bytes.Cut(xmlBytes, []byte(marker))
	if !ok {
		return fmt.Errorf("title %d section %s not found: %w", title, sec, errInput)
	}
	start := bytes.LastIndex(before, []byte("<section"))
	end := sectionEnd(xmlBytes, start)
	if start < 0 || end < 0 {
		return fmt.Errorf("title %d section %s: no element bounds: %w", title, sec, errInput)
	}
	frag := xmlBytes[start:end]
	name := fmt.Sprintf("t%d-s%s.xml", title, strings.NewReplacer("–", "-", "—", "-").Replace(sec))
	if err := os.WriteFile(filepath.Join(dir, name), frag, fileMode); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	fs := sha256.Sum256(frag)
	_, _ = fmt.Fprintf(os.Stdout, "%s %d %s\n", name, len(frag), hex.EncodeToString(fs[:]))
	return nil
}

// sectionEnd returns the offset just after the </section> that closes the
// section opened at start; sections quoted inside notes nest.
func sectionEnd(b []byte, start int) int {
	depth, pos := 0, start
	open, closing := []byte("<section"), []byte("</section>")
	for pos < len(b) {
		o := bytes.Index(b[pos:], open)
		c := bytes.Index(b[pos:], closing)
		if c < 0 {
			return -1
		}
		if o >= 0 && o < c {
			depth++
			pos += o + len(open)
			continue
		}
		depth--
		pos += c + len(closing)
		if depth == 0 {
			return pos
		}
	}
	return -1
}

func readTitle(zipPath, member string) ([]byte, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", zipPath, err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Base(f.Name), member) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open member: %w", err)
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read member: %w", err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("%s: no %s: %w", zipPath, member, errInput)
}
