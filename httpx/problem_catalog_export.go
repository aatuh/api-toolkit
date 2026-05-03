package httpx

import "sort"

// Definitions returns all problem definitions in deterministic code order.
func (catalog *ProblemCatalog) Definitions() []ProblemDefinition {
	if catalog == nil || len(catalog.definitions) == 0 {
		return nil
	}
	out := make([]ProblemDefinition, 0, len(catalog.definitions))
	for _, definition := range catalog.definitions {
		out = append(out, definition)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
