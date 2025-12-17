package scan

import (
	"errors"
	"fmt"
	"strings"
)

// InjectTag represents a parsed `inject:"..."` struct tag.
type InjectTag struct {
	Provider string
}

// parseInjectorTag parses a raw struct tag value for the `inject` key.
// Supported directives (comma-separated):
// - provider:<FuncName>
func parseInjectorTag(raw string) (InjectTag, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return InjectTag{}, nil
	}

	var out InjectTag
	parts, err := splitDirectives(raw)
	if err != nil {
		return out, fmt.Errorf("failed to split directives %q: %w", raw, err)
	}

	for _, part := range parts {
		key, val, ok := cutKV(part)
		if !ok {
			return InjectTag{}, errors.New("invalid injector tag directive")
		}

		switch key {
		case "provider":
			if val == "" {
				return InjectTag{}, errors.New("provider requires a value")
			}
			if out.Provider != "" {
				return InjectTag{}, errors.New("provider already set")
			}
			out.Provider = val
		default:
			return InjectTag{}, errors.New("unknown injector tag directive")
		}
	}

	return out, nil
}
