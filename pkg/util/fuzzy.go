package util

import (
	"strings"
)

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	s1Lower := strings.ToLower(s1)
	s2Lower := strings.ToLower(s2)

	m, n := len(s1Lower), len(s2Lower)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	// Create distance matrix
	d := make([][]int, m+1)
	for i := range d {
		d[i] = make([]int, n+1)
	}

	// Initialize first row and column
	for i := 0; i <= m; i++ {
		d[i][0] = i
	}
	for j := 0; j <= n; j++ {
		d[0][j] = j
	}

	// Calculate distances
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 0
			if s1Lower[i-1] != s2Lower[j-1] {
				cost = 1
			}

			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}

	return d[m][n]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// FindSimilarStrings finds strings similar to the target using fuzzy matching
// Returns up to maxResults strings sorted by similarity (most similar first)
func FindSimilarStrings(target string, candidates []string, maxResults int) []string {
	if len(candidates) == 0 {
		return nil
	}

	type match struct {
		str      string
		distance int
	}

	var matches []match
	targetLower := strings.ToLower(target)

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		candidateLower := strings.ToLower(candidate)

		// Exact match (case-insensitive) - shouldn't happen but just in case
		if targetLower == candidateLower {
			continue
		}

		// Calculate similarity score
		distance := levenshteinDistance(target, candidate)

		// Only include if reasonably similar (distance <= half the length of target, or <= 3)
		maxDistance := len(target) / 2
		if maxDistance < 3 {
			maxDistance = 3
		}

		// Also include if it's a substring match
		isSubstring := strings.Contains(candidateLower, targetLower) || strings.Contains(targetLower, candidateLower)

		if distance <= maxDistance || isSubstring {
			matches = append(matches, match{str: candidate, distance: distance})
		}
	}

	// Sort by distance (most similar first)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].distance < matches[i].distance {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// Return up to maxResults
	var results []string
	for i := 0; i < len(matches) && i < maxResults; i++ {
		results = append(results, matches[i].str)
	}

	return results
}
