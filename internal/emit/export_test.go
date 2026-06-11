package emit

import (
	"bytes"
	"testing"

	"github.com/mickamy/injector/internal/plan"
)

// RenderForTest collects imports for p into the supplied tracker (whose
// reserved aliases the caller has already set up) and renders just the
// container constructor body. It is meant for tests that need to inject
// reserved import names without round-tripping through Emit.
func RenderForTest(t *testing.T, im *Imports, p plan.Plan) string {
	t.Helper()
	collectImports(im, p)
	var buf bytes.Buffer
	if err := writeContainer(&buf, im, p); err != nil {
		t.Fatalf("writeContainer: %v", err)
	}
	return buf.String()
}
