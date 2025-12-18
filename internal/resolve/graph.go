package resolve

import (
	"fmt"
	"go/types"
	"strings"
)

// BuildGraph resolves dependencies starting from container fields.
func BuildGraph(fields []ContainerField, providers []*Provider) (*Graph, error) {
	byType := indexProvidersByType(providers)
	byName, err := indexProvidersByNameStrict(providers)
	if err != nil {
		return nil, err
	}

	overrides, err := collectOverrides(fields, byName)
	if err != nil {
		return nil, fmt.Errorf("resolve: failed to collect overrides: %w", err)
	}

	// seen tracks providers that have already been fully resolved.
	// It is used to avoid re-resolving the same provider multiple times
	// and to allow shared nodes in the dependency graph (DAG).
	seen := map[*Provider]bool{}

	// stack tracks providers that are currently being resolved
	// in the active DFS path. It is used to detect circular dependencies.
	stack := map[*Provider]bool{}

	var roots []*Node
	for _, f := range fields {
		if f.Name == "_" {
			// override-only
			continue
		}
		n, err := resolveField(f, byType, byName, overrides, seen, stack)
		if err != nil {
			return nil, fmt.Errorf("resolve: failed to resolve field: %w", err)
		}
		roots = append(roots, n)
	}

	return &Graph{Roots: roots}, nil
}

func resolveField(
	f ContainerField,
	byType map[string][]*Provider,
	byName map[string]*Provider,
	overrides map[string]*Provider,
	seen map[*Provider]bool,
	stack map[*Provider]bool,
) (*Node, error) {
	var p *Provider

	if f.Inject.Provider != "" {
		var err error
		p, err = lookupProviderByDirective(byName, f.Inject.Provider)
		if err != nil {
			return nil, err
		}
	} else if o, ok := overrides[typeKey(f.Type)]; ok {
		p = o
	} else {
		candidates := byType[typeKey(f.Type)]
		if len(candidates) == 0 {
			return nil, fmt.Errorf("no provider for %s", typeString(f.Type))
		}
		if len(candidates) > 1 {
			return nil, fmt.Errorf("multiple providers for %s", typeString(f.Type))
		}
		p = candidates[0]
	}

	// Ensure return type matches the requested field type when provider is explicitly selected.
	if f.Inject.Provider != "" && !types.Identical(p.ResultType, f.Type) {
		return nil, fmt.Errorf(
			"provider %s returns %s, but field %s requires %s",
			providerString(p),
			typeString(p.ResultType),
			f.Name,
			typeString(f.Type),
		)
	}

	return resolveProvider(p, byType, byName, overrides, seen, stack)
}

func resolveProvider(
	p *Provider,
	byType map[string][]*Provider,
	byName map[string]*Provider,
	overrides map[string]*Provider,
	seen map[*Provider]bool,
	stack map[*Provider]bool,
) (*Node, error) {
	if stack[p] {
		return nil, fmt.Errorf("circular dependency detected at %s", providerString(p))
	}
	if seen[p] {
		// Note: returning a shallow node is OK for now.
		// Topological ordering will be generated later from providers, not from node identity.
		return &Node{Provider: p}, nil
	}

	stack[p] = true
	defer delete(stack, p)
	seen[p] = true

	var deps []*Node
	for _, t := range p.Params {
		key := typeKey(t)

		var dp *Provider
		if o, ok := overrides[key]; ok {
			dp = o
		} else {
			cands := byType[key]
			if len(cands) == 0 {
				return nil, fmt.Errorf("no provider for %s (required by %s)", typeString(t), providerString(p))
			}
			if len(cands) > 1 {
				return nil, fmt.Errorf("multiple providers for %s (required by %s)", typeString(t), providerString(p))
			}
			dp = cands[0]
		}

		n, err := resolveProvider(dp, byType, byName, overrides, seen, stack)
		if err != nil {
			return nil, err
		}
		deps = append(deps, n)
	}

	return &Node{
		Provider: p,
		Deps:     deps,
	}, nil
}

func collectOverrides(fields []ContainerField, byName map[string]*Provider) (map[string]*Provider, error) {
	out := map[string]*Provider{}
	for _, f := range fields {
		if f.Name != "_" {
			continue
		}
		if f.Inject.Provider == "" {
			continue
		}
		p, err := lookupProviderByDirective(byName, f.Inject.Provider)
		if err != nil {
			return nil, fmt.Errorf("override: %v", err)
		}
		out[typeKey(f.Type)] = p
	}
	return out, nil
}

func lookupProviderByDirective(byName map[string]*Provider, directive string) (*Provider, error) {
	if p, ok := byName[directive]; ok {
		return p, nil
	}

	if i := strings.LastIndexByte(directive, '.'); i >= 0 && i+1 < len(directive) {
		short := directive[i+1:]
		if p, ok := byName[short]; ok {
			return p, nil
		}
	}

	return nil, fmt.Errorf("provider %q not found", directive)
}

func indexProvidersByType(ps []*Provider) map[string][]*Provider {
	m := map[string][]*Provider{}
	for _, p := range ps {
		key := typeKey(p.ResultType)
		m[key] = append(m[key], p)
	}
	return m
}

func indexProvidersByNameStrict(ps []*Provider) (map[string]*Provider, error) {
	m := map[string]*Provider{}
	var conflicts []string

	for _, p := range ps {
		if p.Name == "" {
			continue
		}
		if existing, ok := m[p.Name]; ok {
			conflicts = append(
				conflicts,
				fmt.Sprintf("resolve: %s %s conflicts with %s", p.Name, providerString(existing), providerString(p)),
			)
			continue
		}
		m[p.Name] = p
	}

	if len(conflicts) > 0 {
		return nil, fmt.Errorf("resolve: provider name conflicts:\n%s", strings.Join(conflicts, "\n"))
	}
	return m, nil
}

func typeKey(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == nil {
			return ""
		}
		return p.Path()
	})
}

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == nil {
			return ""
		}
		// Use full path to reduce ambiguity in errors.
		return p.Path()
	})
}

func providerString(p *Provider) string {
	if p == nil {
		return "<nil>"
	}
	if p.PkgPath == "" {
		return p.Name
	}
	return p.PkgPath + "." + p.Name
}
