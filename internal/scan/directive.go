package scan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mickamy/injector/internal/ir"
)

// directiveTag is the prefix that marks an //injector:container line.
const directiveTag = "injector:container"

// ParsedDirective captures the values parsed from one or more comment lines
// containing //injector:container ... .
//
// All fields are optional. ReturnsExpr is the raw textual type expression
// (e.g. "greeter.Greeter") and is resolved to a concrete types.Type later by
// the caller (which has access to the surrounding type-checked package).
type ParsedDirective struct {
	// Found is true when at least one //injector:container line was seen.
	Found bool
	// Name overrides the constructor name. Empty means not specified.
	Name string
	// ReturnsExpr is the textual return type. Empty means not specified.
	ReturnsExpr string
	// Must is the parsed must mode. MustUnset means not specified.
	Must ir.MustMode
}

// ParseDirective scans the given comment lines for //injector:container and
// returns the parsed values together with any error messages.
//
// Comment lines may include the leading "//" or "// "; both forms are
// accepted. At most one //injector:container line is allowed per call;
// duplicates produce an error message.
func ParseDirective(commentLines []string) (ParsedDirective, []string) {
	var (
		pd   ParsedDirective
		errs []string
	)

	for _, line := range commentLines {
		body, ok := stripCommentMarker(line)
		if !ok {
			continue
		}
		rest, ok := stripDirectivePrefix(body)
		if !ok {
			continue
		}
		if pd.Found {
			errs = append(errs, "duplicate //injector:container directive")
			continue
		}
		pd.Found = true

		for tok := range strings.FieldsSeq(rest) {
			if err := applyDirectiveToken(&pd, tok); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	return pd, errs
}

// stripCommentMarker removes a leading "//" (and any surrounding whitespace)
// from a comment line.
func stripCommentMarker(line string) (string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "//") {
		return "", false
	}
	s = strings.TrimPrefix(s, "//")
	return strings.TrimSpace(s), true
}

// stripDirectivePrefix matches "injector:container" exactly or followed by
// whitespace, returning the trailing arguments.
func stripDirectivePrefix(body string) (string, bool) {
	if body == directiveTag {
		return "", true
	}
	rest, ok := strings.CutPrefix(body, directiveTag+" ")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

func applyDirectiveToken(pd *ParsedDirective, tok string) error {
	if tok == "" {
		return nil
	}

	key, value, hasEq := strings.Cut(tok, "=")

	switch key {
	case "name":
		if !hasEq || value == "" {
			return errors.New("directive name= requires a value")
		}
		if pd.Name != "" {
			return errors.New("directive name= specified more than once")
		}
		pd.Name = value
	case "returns":
		if !hasEq || value == "" {
			return errors.New("directive returns= requires a value")
		}
		if pd.ReturnsExpr != "" {
			return errors.New("directive returns= specified more than once")
		}
		pd.ReturnsExpr = value
	case "must":
		if pd.Must != ir.MustUnset {
			return errors.New("directive must specified more than once")
		}
		if !hasEq {
			pd.Must = ir.MustOn
			return nil
		}
		switch value {
		case "true":
			pd.Must = ir.MustOn
		case "false":
			pd.Must = ir.MustOff
		default:
			return fmt.Errorf("directive must= must be true or false, got %q", value)
		}
	default:
		return fmt.Errorf("unknown directive key %q", key)
	}
	return nil
}
