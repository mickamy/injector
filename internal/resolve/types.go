package resolve

import "go/types"

// Graph represents a resolved dependency graph.
type Graph struct {
	Roots []*Node
}

// Node represents a node in the resolved dependency graph.
//
// Each node corresponds to exactly one Provider.
// Nodes may be shared across the graph to represent a DAG
// (i.e. the same Provider can be depended on by multiple parents).
type Node struct {
	Provider *Provider
	Deps     []*Node
}

// Provider represents a constructor function that can produce a value
// for dependency injection.
//
// A Provider is identified by its identity (pointer equality), not by value.
// It is treated as immutable during resolution and is shared across the
// dependency graph for cycle detection and override resolution.
type Provider struct {
	PkgPath     string
	Name        string
	ResultType  types.Type
	ReturnError bool
	Params      []types.Type
	Position    string
}

// ContainerField represents an injectable field in a Container struct.
//
// Note:
// - Name can be "_" for override-only fields.
// - Type is the type to be provided (e.g. service.User, infra.Database).
type ContainerField struct {
	Name   string
	Type   types.Type
	Inject InjectTag
}

// InjectTag is a parsed `inject` struct tag for Container fields.
type InjectTag struct {
	// Provider selects a specific provider function by name.
	// Example: `inject:"provider:service.NewUser"`
	Provider string
}
