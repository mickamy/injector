package packages_test

import (
	"strings"
	"testing"

	"github.com/mickamy/injector/internal/packages"
)

func TestLoad_NoPatterns(t *testing.T) {
	t.Parallel()

	_, err := packages.Load(nil, packages.Config{})
	if err == nil {
		t.Fatal("Load(nil) returned nil error")
	}
	if !strings.Contains(err.Error(), "no package patterns") {
		t.Errorf("error = %q, want substring %q", err, "no package patterns")
	}
}
