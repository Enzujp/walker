package walker

import "fmt"

// Endpoint is the identity of a route, excluding documentation metadata.
type Endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// Changes reports additions and removals, each sorted by path and then method.
// Method/path changes appear as a removal plus an addition; no renames are inferred.
type Changes struct {
	Added   []Endpoint `json:"added"`
	Removed []Endpoint `json:"removed"`
}

// Diff compares endpoint identities. Descriptions, examples, auth and other
// metadata do not affect the result. Both manifests are validated before comparison.
func Diff(previous, current []Route) (Changes, error) {
	before, err := Normalize(previous)
	if err != nil {
		return Changes{}, fmt.Errorf("previous routes: %w", err)
	}
	after, err := Normalize(current)
	if err != nil {
		return Changes{}, fmt.Errorf("current routes: %w", err)
	}
	result := Changes{Added: []Endpoint{}, Removed: []Endpoint{}}
	previousSet, currentSet := map[Endpoint]bool{}, map[Endpoint]bool{}
	for _, r := range before {
		previousSet[Endpoint{r.Method, r.Path}] = true
	}
	for _, r := range after {
		currentSet[Endpoint{r.Method, r.Path}] = true
	}
	for _, r := range before {
		e := Endpoint{r.Method, r.Path}
		if !currentSet[e] {
			result.Removed = append(result.Removed, e)
		}
	}
	for _, r := range after {
		e := Endpoint{r.Method, r.Path}
		if !previousSet[e] {
			result.Added = append(result.Added, e)
		}
	}
	return result, nil
}
