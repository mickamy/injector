package resolve

import (
	"go/types"

	"github.com/mickamy/injector/internal/scan"
)

// EmbeddedContainerRef represents a reference to an embedded container.
type EmbeddedContainerRef struct {
	ContainerPkgPath string
	ContainerName    string
	Type             types.Type // *Container or Container
	FuncName         string     // "NewInfra"
	ReturnError      bool
	ExportedFields   []ExportedField
}

// ExportedField represents an exported field of an embedded container.
type ExportedField struct {
	Name string     // "KVS"
	Type types.Type // *kvs.KVS
}

// DetectEmbeddedContainers finds embedded container references among
// the container fields and returns their metadata.
//
// registry maps "pkgPath.Name" to the corresponding ContainerSpec.
// processedResults maps the same key to whether the container's generated
// constructor returns an error.
func DetectEmbeddedContainers(
	fields []scan.ContainerField,
	registry map[string]scan.ContainerSpec,
	processedResults map[string]bool,
) []EmbeddedContainerRef {
	var refs []EmbeddedContainerRef

	for _, f := range fields {
		if !f.IsEmbeddedCandidate {
			continue
		}
		if f.Type == nil {
			continue
		}

		// Unwrap pointer if needed.
		t := f.Type
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}

		named, ok := t.(*types.Named)
		if !ok {
			continue
		}

		obj := named.Obj()
		if obj == nil || obj.Pkg() == nil {
			continue
		}

		key := obj.Pkg().Path() + "." + obj.Name()
		spec, ok := registry[key]
		if !ok {
			continue
		}

		// Collect exported fields from the container spec.
		// Exported fields are non-"_" fields that have an inject tag.
		var exported []ExportedField
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := range st.NumFields() {
			sf := st.Field(i)
			if !sf.Exported() {
				continue
			}
			// Check that this field is actually an inject-tagged field in the spec
			// (not just any exported struct field).
			if !isInjectField(spec, sf.Name()) {
				continue
			}
			exported = append(exported, ExportedField{
				Name: sf.Name(),
				Type: sf.Type(),
			})
		}

		if len(exported) == 0 {
			continue
		}

		funcName := "New" + upperFirst(spec.Name)
		returnErr := false
		if re, ok := processedResults[key]; ok {
			returnErr = re
		}

		refs = append(refs, EmbeddedContainerRef{
			ContainerPkgPath: spec.PkgPath,
			ContainerName:    spec.Name,
			Type:             named,
			FuncName:         funcName,
			ReturnError:      returnErr,
			ExportedFields:   exported,
		})
	}

	return refs
}

// isInjectField checks whether the named field appears as a non-"_"
// inject-tagged field in the container spec.
func isInjectField(spec scan.ContainerSpec, fieldName string) bool {
	for _, f := range spec.Fields {
		if f.Name == fieldName && !f.IsEmbeddedCandidate {
			return true
		}
	}
	return false
}

// CreateSyntheticProviders creates synthetic providers for the given
// embedded container references:
//  1. A constructor provider: NewInfra() → *Infra
//  2. Field-access providers: *Infra → infra.KVS (one per exported field)
func CreateSyntheticProviders(refs []EmbeddedContainerRef) []*Provider {
	var out []*Provider

	for _, ref := range refs {
		ptrType := types.NewPointer(ref.Type)

		// Parameter provider: the embedded container is passed as
		// a function argument rather than constructed internally.
		out = append(out, &Provider{
			PkgPath:     ref.ContainerPkgPath,
			Name:        ref.ContainerName,
			NameWithPkg: ref.ContainerPkgPath + "." + ref.ContainerName,
			ResultType:  ptrType,
			IsParam:     true,
		})

		// Field-access providers.
		for _, ef := range ref.ExportedFields {
			out = append(out, &Provider{
				Name:        ref.ContainerName + "." + ef.Name,
				NameWithPkg: ref.ContainerPkgPath + "." + ref.ContainerName + "." + ef.Name,
				ResultType:  ef.Type,
				Params:      []types.Type{ptrType},
				FieldAccess: ef.Name,
			})
		}
	}

	return out
}

// MergeProviders merges regular and synthetic providers.
//   - Field-access synthetic providers override regular providers for the same result type.
//   - Synthetic constructor providers are skipped if a regular provider with
//     the same NameWithPkg already exists (e.g. from a previously generated file).
func MergeProviders(regular, synthetic []*Provider) []*Provider {
	if len(synthetic) == 0 {
		return regular
	}

	// Collect types produced by field-access synthetic providers.
	fieldAccessTypes := make(map[string]struct{})
	for _, p := range synthetic {
		if p.FieldAccess != "" {
			fieldAccessTypes[typeKey(p.ResultType)] = struct{}{}
		}
	}

	// Index regular providers by NameWithPkg.
	regularByName := make(map[string]struct{})
	for _, p := range regular {
		if p.NameWithPkg != "" {
			regularByName[p.NameWithPkg] = struct{}{}
		}
	}

	// Filter regular providers: remove those overridden by field-access providers.
	var out []*Provider
	for _, p := range regular {
		key := typeKey(p.ResultType)
		if _, ok := fieldAccessTypes[key]; ok {
			continue // overridden by field-access provider
		}
		out = append(out, p)
	}

	// Add synthetic providers, skipping constructors that already exist.
	for _, p := range synthetic {
		if p.FieldAccess == "" {
			// Constructor provider: skip if already discovered as a regular provider.
			if _, ok := regularByName[p.NameWithPkg]; ok {
				continue
			}
		}
		out = append(out, p)
	}

	return out
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}
