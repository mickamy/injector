package resolve

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mickamy/injector/internal/scan"
)

// ConvertContainerFields converts a scanned ContainerSpec into resolve-ready fields.
//
// Resolution target rules:
//   - If the Container has at least one `inject`-marked *public field* (non-blank), only those marked fields are included.
//   - Blank fields ("_") are treated as provider overrides and do NOT switch the container into explicit mode.
//     They are included only when they are `inject`-marked.
func ConvertContainerFields(c scan.ContainerSpec) ([]ContainerField, error) {
	var errs []string

	for _, f := range c.Fields {
		if f.Name == "_" {
			continue
		}
	}

	var out []ContainerField
	for _, f := range c.Fields {
		// Blank field: override only, include only if marked.
		if f.Name == "_" {
			if !isMarkedField(f) {
				continue
			}
		}

		if f.Type == nil {
			errs = append(errs, fmt.Sprintf("resolve: %s type information is missing for field %s", f.Position, f.Name))
			continue
		}

		out = append(out, ContainerField{
			Name: f.Name,
			Type: f.Type,
			Inject: InjectTag{
				Provider: f.Inject.Provider,
			},
		})
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return out, nil
}

// ConvertProviders converts scanned ProviderDecl into resolve Provider nodes.
func ConvertProviders(ps []scan.ProviderSpec) ([]*Provider, error) {
	var out []*Provider
	var errs []string

	for _, p := range ps {
		if p.ResultType == nil {
			errs = append(errs, fmt.Sprintf("resolve: %s type information is missing for provider %s", p.Position, p.Name))
			continue
		}

		out = append(out, &Provider{
			PkgPath:    p.PkgPath,
			Name:       p.Name,
			ResultType: p.ResultType,
			Params:     p.Params,
			Position:   p.Position,
		})
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return out, nil
}

func isMarkedField(f scan.ContainerField) bool {
	// Marker-only: InjectRaw can be empty. We rely on the presence of the `inject` marker.
	return hasInjectMarkerInRaw(f.TagRaw) || f.InjectRaw != ""
}

func hasInjectMarkerInRaw(tagRaw string) bool {
	parts := strings.Fields(tagRaw)
	for _, p := range parts {
		if p == "inject" {
			return true
		}
	}
	return false
}
