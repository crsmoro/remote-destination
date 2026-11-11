package api

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"waze-remoto/internal/destination"
)

func positionReference() (float64, float64, string) {
	fallback := func() (float64, float64, string) { return -26.3045, -48.8487, "joinville" }
	path := os.Getenv("WAZE_POSITION_FILE")
	if !filepath.IsAbs(path) {
		return fallback()
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) > 5*time.Minute || time.Since(info.ModTime()) < -time.Minute {
		return fallback()
	}
	f, err := os.Open(path)
	if err != nil {
		return fallback()
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1025))
	if err != nil || len(data) > 1024 {
		return fallback()
	}
	var p struct {
		Available bool     `json:"available"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if json.Unmarshal(data, &p) != nil || !p.Available || p.Latitude == nil || p.Longitude == nil || math.IsNaN(*p.Latitude) || math.IsNaN(*p.Longitude) || math.Abs(*p.Latitude) > 90 || math.Abs(*p.Longitude) > 180 {
		return fallback()
	}
	return *p.Latitude, *p.Longitude, "gps_unverified_age"
}

func normalizedWords(s string) []string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

func streetType(s string) string {
	switch s {
	case "rua", "r":
		return "rua"
	case "avenida", "av":
		return "avenida"
	case "estrada", "est":
		return "estrada"
	case "rodovia", "rod":
		return "rodovia"
	case "travessa", "tv":
		return "travessa"
	case "alameda", "al":
		return "alameda"
	case "praca", "pca":
		return "praca"
	}
	return ""
}

func isStreetPrefix(s string) bool { return streetType(s) != "" }

// Compare street names separately from house numbers and locality. A number or
// city match alone must never identify a street.
func streetCandidateMatches(input, label string) bool {
	inputParts := strings.Split(input, ",")
	want := normalizedWords(inputParts[0])
	// The next comma-delimited field may contain the house number.
	if len(inputParts) > 1 {
		field := normalizedWords(inputParts[1])
		if len(field) == 1 && strings.Trim(field[0], "0123456789") == "" {
			want = append(want, field[0])
		}
	}
	parts := strings.Split(label, ",")
	if len(want) < 2 || len(parts) == 0 {
		return false
	}
	have := normalizedWords(parts[0])
	if len(have) == 0 || streetType(want[0]) == "" || streetType(want[0]) != streetType(have[0]) {
		return false
	}
	ignore := map[string]bool{"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true}
	isNumber := func(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }
	streetWords := map[string]bool{}
	for _, w := range have[1:] {
		if !ignore[w] && !isNumber(w) {
			streetWords[w] = true
		}
	}
	numbers := map[string]bool{}
	// A formatted address normally places the house number in the next field.
	for _, w := range have[1:] {
		if isNumber(w) {
			numbers[w] = true
		}
	}
	if len(parts) > 1 {
		for _, w := range normalizedWords(parts[1]) {
			if isNumber(w) {
				numbers[w] = true
			}
		}
	}
	matched := 0
	for _, w := range want[1:] {
		if ignore[w] {
			continue
		}
		if isNumber(w) {
			if !numbers[w] {
				return false
			}
			continue
		}
		if !streetWords[w] {
			return false
		}
		matched++
		delete(streetWords, w)
	}
	return matched > 0 && len(streetWords) == 0
}

// For multiword queries reject results sharing no meaningful word. This is
// only a weak incompatibility check, not proof of identity or a global search.
func plausibleAutomaticCandidate(input string, c destination.Candidate) bool {
	want := normalizedWords(input)
	if c.GoogleName != "" {
		if len(want) > 0 && isStreetPrefix(want[0]) {
			return streetCandidateMatches(input, c.GoogleAddress)
		}
		name := normalizedWords(c.GoogleName)
		city := normalizedWords(c.City)
		// Locality can qualify a query, but is never evidence of the name.
		if len(city) > 0 && len(want) > len(city) && strings.Join(want[len(want)-len(city):], " ") == strings.Join(city, " ") {
			want = want[:len(want)-len(city)]
			if len(want) > 0 && want[len(want)-1] == "em" {
				want = want[:len(want)-1]
			}
		}
		return len(want) > 0 && strings.Join(want, " ") == strings.Join(name, " ")
	}
	if len(want) > 0 && isStreetPrefix(want[0]) {
		return streetCandidateMatches(input, c.Label)
	}
	// A formatted street address cannot establish the identity of a business,
	// even if its city or street happens to share a word with the query.
	if c.AddressOnlyGoogle {
		return false
	}
	if len(want) > 0 && strings.Contains(" "+strings.Join(normalizedWords(c.Label), " ")+" ", " "+strings.Join(want, " ")+" ") {
		return true
	}
	generic := map[string]bool{"farmacia": true, "drogaria": true, "mercado": true, "supermercado": true, "padaria": true, "restaurante": true, "rua": true, "avenida": true, "de": true, "da": true, "do": true, "em": true}
	have := normalizedWords(c.Label + " " + c.City)
	for _, w := range want {
		if len([]rune(w)) < 3 || (len(want) > 1 && generic[w]) {
			continue
		}
		for _, h := range have {
			if w == h {
				return true
			}
		}
	}
	return false
}

// A match is only a hint: the same words can name a business, not a city.
// Keep every candidate and tell the user the reference is uncertain.
func prioritizePossibleCity(c []destination.Candidate, input string) bool {
	normalize := func(s string) string {
		var b strings.Builder
		for _, r := range norm.NFD.String(strings.ToLower(s)) {
			if unicode.Is(unicode.Mn, r) {
				continue
			}
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			} else {
				b.WriteByte(' ')
			}
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	text := " " + normalize(input) + " "
	matched := map[string]bool{}
	for _, v := range c {
		city := normalize(v.City)
		if city != "" && strings.Contains(text, " "+city+" ") {
			matched[city] = true
		}
	}
	if len(matched) != 1 {
		return false
	}
	sort.SliceStable(c, func(i, j int) bool { return matched[normalize(c[i].City)] && !matched[normalize(c[j].City)] })
	return true
}

func rankCandidates(c []destination.Candidate, lat, lon float64) {
	distance := func(v destination.Candidate) float64 {
		const radians = math.Pi / 180
		dlat := (v.Latitude - lat) * radians
		dlon := (v.Longitude - lon) * radians
		return math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat*radians)*math.Cos(v.Latitude*radians)*math.Pow(math.Sin(dlon/2), 2)
	}
	sort.SliceStable(c, func(i, j int) bool { return distance(c[i]) < distance(c[j]) })
}
