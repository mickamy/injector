package scan

import (
	"errors"
	"fmt"
	"strings"
)

// InjectorTag represents a parsed `inject:"..."` struct tag.
type InjectorTag struct {
	Provider string
}

// parseInjectorTag parses a raw struct tag value for the `inject` key.
// Supported directives (comma-separated):
// - provider:<FuncName>
func parseInjectorTag(raw string) (InjectorTag, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return InjectorTag{}, nil
	}

	var out InjectorTag
	parts, err := splitDirectives(raw)
	if err != nil {
		return out, fmt.Errorf("failed to split directives %q: %w", raw, err)
	}

	for _, part := range parts {
		key, val, ok := cutKV(part)
		if !ok {
			return InjectorTag{}, errors.New("invalid injector tag directive")
		}

		switch key {
		case "provider":
			if val == "" {
				return InjectorTag{}, errors.New("provider requires a value")
			}
			if out.Provider != "" {
				return InjectorTag{}, errors.New("provider already set")
			}
			out.Provider = val
		default:
			return InjectorTag{}, errors.New("unknown injector tag directive")
		}
	}

	return out, nil
}
