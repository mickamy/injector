package resolve

import (
	"errors"
	"fmt"
	"go/types"
	"strings"

	"github.com/mickamy/injector/internal/scan"
)

// ConvertContainerFields converts a scanned ContainerSpec into resolve-ready fields.
//
// Resolution target rules:
//   - If the Container has at least one `inject`-marked *public field* (non-blank), only those marked fields are included.
//   - Blank fields ("_") are treated as provider overrides and do NOT switch the container into explicit mode.
//     They are included only when they are `inject`-marked.
//
// containerRegistry is used to distinguish container-type inject:"param" fields
// (handled by DetectEmbeddedContainers) from non-container-type inject:"param"
// fields (included as regular fields resolved by IsParam providers).
func ConvertContainerFields(c scan.ContainerSpec, containerRegistry map[string]scan.ContainerSpec) ([]ContainerField, error) {
	var errs []string

	for _, f := range c.Fields {
		if f.Name == "_" {
			continue
		}
	}

	var out []ContainerField
	for _, f := range c.Fields {
		// Container-type embedded candidates are handled separately
		// by DetectEmbeddedContainers. Non-container inject:"param"
		// fields are kept as regular fields.
		if f.IsEmbeddedCandidate {
			if isContainerType(f.Type, containerRegistry) {
				continue
			}
		}
		// Returns fields define the return type, not an injectable field.
		if f.IsReturns {
			continue
		}
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
			PkgPath:     p.PkgPath,
			Name:        p.Name,
			NameWithPkg: strings.Join([]string{p.PkgPath, p.Name}, "."),
			ResultType:  p.ResultType,
			ReturnError: p.ReturnError,
			Params:      p.Params,
			Position:    p.Position,
		})
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return out, nil
}

// FilterOutSelf removes providers that match the container's own generated
// constructor names (e.g. NewFoo, MustNewFoo) to prevent conflicts during
// re-generation while keeping other containers' constructors available.
func FilterOutSelf(providers []*Provider, pkgPath string, names []string) []*Provider {
	exclude := make(map[string]struct{}, len(names))
	for _, n := range names {
		exclude[pkgPath+"."+n] = struct{}{}
	}
	out := make([]*Provider, 0, len(providers))
	for _, p := range providers {
		if _, ok := exclude[p.NameWithPkg]; ok {
			continue
		}
		out = append(out, p)
	}
	return out
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

// isContainerType checks whether the given type references a container in the registry.
func isContainerType(t types.Type, registry map[string]scan.ContainerSpec) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	key := obj.Pkg().Path() + "." + obj.Name()
	_, ok = registry[key]
	return ok
}
