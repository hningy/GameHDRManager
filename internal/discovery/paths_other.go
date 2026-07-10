//go:build !windows

package discovery

func ScanInstalled() ([]Candidate, error) { return []Candidate{}, nil }
