package resolve

import "fmt"

// OrderProviders returns providers in topological order (dependencies first).
// Providers may appear only once even if referenced multiple times by roots.
func OrderProviders(g *Graph) ([]*Provider, error) {
	if g == nil {
		return nil, fmt.Errorf("resolve: graph is nil")
	}

	visited := map[*Provider]struct{}{}
	onstack := map[*Provider]struct{}{}
	var out []*Provider

	var visit func(n *Node) error
	visit = func(n *Node) error {
		if n == nil || n.Provider == nil {
			return nil
		}

		p := n.Provider
		if _, ok := onstack[p]; ok {
			return fmt.Errorf("resolve: circular dependency detected at %s", providerString(p))
		}
		if _, ok := visited[p]; ok {
			return nil
		}

		onstack[p] = struct{}{}
		for _, d := range n.Deps {
			if err := visit(d); err != nil {
				return err
			}
		}
		delete(onstack, p)

		visited[p] = struct{}{}
		out = append(out, p)
		return nil
	}

	for _, r := range g.Roots {
		if err := visit(r); err != nil {
			return nil, err
		}
	}

	return out, nil
}
