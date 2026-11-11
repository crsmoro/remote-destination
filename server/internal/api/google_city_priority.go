package api

import (
	"sort"
	"strings"
	"waze-remoto/internal/destination"
)

// A returned locality at the end is only a clue, not proof: it may be part
// of a business name. An exact Google display name takes precedence.
func inferredGoogleCity(candidates []destination.Candidate, input string) string {
	words := normalizedWords(input)
	full := strings.Join(words, " ")
	if full == "" {
		return ""
	}
	for _, c := range candidates {
		if len(words) > 1 && c.GoogleName != "" && strings.Join(normalizedWords(c.GoogleName), " ") == full {
			return ""
		}
	}
	best := ""
	for _, c := range candidates {
		city := strings.Join(normalizedWords(c.City), " ")
		if city == "" {
			continue
		}
		matches := full == city || strings.HasSuffix(full, " "+city)
		state := strings.Join(normalizedWords(c.State), " ")
		if !matches && state != "" {
			suffix := city + " " + state
			matches = full == suffix || strings.HasSuffix(full, " "+suffix)
		}
		if matches && len(city) > len(best) {
			best = city
		}
	}
	return best
}

// Stable partition retains every provider result and its relative order.
func prioritizeGoogleCity(candidates []destination.Candidate, city string) {
	target := strings.Join(normalizedWords(city), " ")
	if target == "" {
		return
	}
	match := func(c destination.Candidate) bool { return strings.Join(normalizedWords(c.City), " ") == target }
	sort.SliceStable(candidates, func(i, j int) bool { return match(candidates[i]) && !match(candidates[j]) })
}
