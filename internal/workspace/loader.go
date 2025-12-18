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
