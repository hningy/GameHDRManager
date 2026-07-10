package discovery

import (
	"sort"
	"strings"
)

// MergeCandidates removes the same title discovered from multiple roots while
// retaining deterministic ordering for the UI.
func MergeCandidates(groups ...[]Candidate) []Candidate {
	unique := make(map[string]Candidate)
	for _, group := range groups {
		for _, candidate := range group {
			key := strings.ToLower(candidate.Source + "|" + candidate.Launch.Value + "|" + candidate.Name)
			if _, exists := unique[key]; !exists {
				unique[key] = candidate
			}
		}
	}
	merged := make([]Candidate, 0, len(unique))
	for _, candidate := range unique {
		merged = append(merged, candidate)
	}
	sort.Slice(merged, func(i, j int) bool { return strings.ToLower(merged[i].Name) < strings.ToLower(merged[j].Name) })
	return merged
}
