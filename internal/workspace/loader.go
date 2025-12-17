package workspace

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// LoadConfig configures how packages are loaded.
type LoadConfig struct {
	// BuildTags are passed to `go list` via `-tags`.
	BuildTags []string

	// Tests includes test files/packages when true.
	Tests bool
}

// Loaded holds loaded packages and the effective config.
type Loaded struct {
	Packages []*packages.Package
}

// Load loads Go packages for the given patterns (e.g. ./...).
func Load(patterns []string, cfg LoadConfig) (*Loaded, error) {
	if len(patterns) == 0 {
		return nil, errors.New("workspace: no package patterns")
	}

	mode := packages.NeedName |
		packages.NeedFiles |
		packages.NeedCompiledGoFiles |
		packages.NeedModule |
		packages.NeedImports |
		packages.NeedDeps |
		packages.NeedTypes |
		packages.NeedTypesInfo |
		packages.NeedSyntax

	pc := &packages.Config{
		Mode:  mode,
		Tests: cfg.Tests,
	}

	if len(cfg.BuildTags) > 0 {
		pc.BuildFlags = []string{fmt.Sprintf("-tags=%s", joinTags(cfg.BuildTags))}
	}

	pkgs, err := packages.Load(pc, patterns...)
	if err != nil {
		return nil, fmt.Errorf("workspace: load packages: %w", err)
	}

	// packages.Load may return pkgs with Errors set even if err == nil.
	if hasErrors(pkgs) {
		return nil, fmt.Errorf("workspace: load packages: %w", formatPkgErrors(pkgs))
	}

	return &Loaded{
		Packages: pkgs,
	}, nil
}

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	nonEmpty := make([]string, 0, len(tags))
	for _, t := range tags {
		if t == "" {
			continue
		}
		nonEmpty = append(nonEmpty, t)
	}

	// go list expects space-separated build tags.
	// The entire string is passed as a single argument to -tags.
	return strings.Join(nonEmpty, " ")
}

func hasErrors(pkgs []*packages.Package) bool {
	for _, p := range pkgs {
		if p == nil {
			continue
		}
		if len(p.Errors) > 0 {
			return true
		}
	}
	return false
}

type pkgErrors struct {
	msg string
}

func (e pkgErrors) Error() string {
	return e.msg
}

func formatPkgErrors(pkgs []*packages.Package) error {
	lines := make([]string, 0, 8)
	for _, p := range pkgs {
		if p == nil {
			continue
		}
		for _, pe := range p.Errors {
			// pe.Pos is already in "file:line:col" when available.
			if pe.Pos != "" {
				lines = append(lines, fmt.Sprintf("%s: %s", pe.Pos, pe.Msg))
				continue
			}
			if p.PkgPath != "" {
				lines = append(lines, fmt.Sprintf("%s: %s", p.PkgPath, pe.Msg))
				continue
			}
			lines = append(lines, pe.Msg)
		}
	}
	if len(lines) == 0 {
		return errors.New("unknown package load error")
	}
	return pkgErrors{msg: joinLines(lines)}
}

func joinLines(lines []string) string {
	if len(lines) == 1 {
		return lines[0]
	}
	n := 0
	for _, s := range lines {
		n += len(s) + 1
	}
	b := make([]byte, 0, n)
	for i, s := range lines {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, s...)
	}
	return string(b)
}
