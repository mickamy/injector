package scan

import (
	"fmt"
	"strings"
)

// TagKind classifies the directive embedded in an inject:"..." struct tag.
type TagKind int

const (
	// TagInvalid is the zero value used for parses that did not succeed.
	TagInvalid TagKind = iota
	// TagMarker corresponds to inject:"" — the bare marker form.
	TagMarker
	// TagWith corresponds to inject:"with=Foo" — explicit provider selection.
	TagWith
	// TagArg corresponds to inject:"arg" or inject:"arg=name" — an input
	// passed in via the constructor.
	TagArg
	// TagReturns corresponds to inject:"returns" — return-type declaration.
	TagReturns
)

// ParsedTag holds the result of parsing the value of an inject:"..." tag.
//
// The role of the field (RoleOut / RoleArg / RoleOverride / RoleReturnsOnly)
// is determined by the caller, which combines the tag with the field name
// ("_" vs. named) and produces a final ir.Field.
type ParsedTag struct {
	Kind TagKind
	// With is the provider reference when Kind == TagWith.
	With string
	// ArgName is the optional input name when Kind == TagArg.
	// Empty means the name should be derived from the field type by the caller.
	ArgName string
}

// ParseTag parses the value of a struct field's inject:"..." tag.
//
// Supported forms:
//
//   - ""              the marker form
//   - "with=<ref>"    explicit provider reference
//   - "arg"           input parameter (name derived from type)
//   - "arg=<name>"    input parameter with explicit name
//   - "returns"       return-type declaration
func ParseTag(value string) (ParsedTag, error) {
	s := strings.TrimSpace(value)

	if s == "" {
		return ParsedTag{Kind: TagMarker}, nil
	}

	key, val, hasEq := strings.Cut(s, "=")

	switch key {
	case "with":
		if !hasEq || val == "" {
			return ParsedTag{}, fmt.Errorf(`inject:"with=..." requires a provider reference`)
		}
		return ParsedTag{Kind: TagWith, With: val}, nil
	case "arg":
		if !hasEq {
			return ParsedTag{Kind: TagArg}, nil
		}
		if val == "" {
			return ParsedTag{}, fmt.Errorf(`inject:"arg=..." requires a name`)
		}
		return ParsedTag{Kind: TagArg, ArgName: val}, nil
	case "returns":
		if hasEq {
			return ParsedTag{}, fmt.Errorf(`inject:"returns" does not take a value`)
		}
		return ParsedTag{Kind: TagReturns}, nil
	default:
		return ParsedTag{}, fmt.Errorf(`unknown inject tag form %q`, s)
	}
}
