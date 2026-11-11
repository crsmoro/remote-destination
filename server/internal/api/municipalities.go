package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"waze-remoto/internal/destination"
)

type municipality struct {
	ID   int    `json:"id"`
	Name string `json:"nome"`
	UF   string `json:"uf"`
}

// ValidateMunicipalityIndex checks a configured snapshot at startup.
func ValidateMunicipalityIndex() error { _, err := municipalityIndex(); return err }

func municipalityIndex() ([]municipality, error) {
	path := os.Getenv("WAZE_MUNICIPIOS_FILE")
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("invalid municipality index")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 2<<20))
	if err != nil || len(b) >= 2<<20 {
		return nil, errors.New("invalid municipality index")
	}
	return validateMunicipalitySnapshot(b)
}

var municipalityUFs = map[int]string{11: "RO", 12: "AC", 13: "AM", 14: "RR", 15: "PA", 16: "AP", 17: "TO", 21: "MA", 22: "PI", 23: "CE", 24: "RN", 25: "PB", 26: "PE", 27: "AL", 28: "SE", 29: "BA", 31: "MG", 32: "ES", 33: "RJ", 35: "SP", 41: "PR", 42: "SC", 43: "RS", 50: "MS", 51: "MT", 52: "GO", 53: "DF"}

func validateMunicipalitySnapshot(b []byte) ([]municipality, error) {
	invalid := errors.New("invalid municipality index")
	if !utf8.Valid(b) || !uniqueJSONKeys(b) {
		return nil, invalid
	}
	var envelope struct {
		Version        int             `json:"version"`
		Count          int             `json:"count"`
		SHA256         string          `json:"sha256"`
		Municipalities json.RawMessage `json:"municipalities"`
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&envelope) != nil || decoder.Decode(new(any)) != io.EOF || envelope.Version != 1 || envelope.Count < 5000 || envelope.Count > 6500 {
		return nil, invalid
	}
	sum := sha256.Sum256(envelope.Municipalities)
	if envelope.SHA256 != hex.EncodeToString(sum[:]) {
		return nil, invalid
	}
	var list []municipality
	decoder = json.NewDecoder(bytes.NewReader(envelope.Municipalities))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&list) != nil || len(list) != envelope.Count {
		return nil, invalid
	}
	seen := make(map[int]bool, len(list))
	states := make(map[string]bool)
	for _, m := range list {
		if m.ID < 1000000 || m.ID > 9999999 || seen[m.ID] || municipalityUFs[m.ID/100000] != m.UF || strings.TrimSpace(m.Name) != m.Name || m.Name == "" || utf8.RuneCountInString(m.Name) > 120 || normalizePlace(m.Name) == "" {
			return nil, invalid
		}
		for _, r := range m.Name {
			if unicode.IsControl(r) || r == unicode.ReplacementChar {
				return nil, invalid
			}
		}
		seen[m.ID] = true
		states[m.UF] = true
	}
	if len(states) != 27 {
		return nil, invalid
	}
	return list, nil
}

// Reject duplicate keys rather than accepting the last occurrence.
func uniqueJSONKeys(b []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(b))
	var value func() error
	value = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return fmt.Errorf("duplicate key")
				}
				seen[k] = true
				if err := value(); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return fmt.Errorf("invalid object")
			}
		case '[':
			for dec.More() {
				if err := value(); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return fmt.Errorf("invalid array")
			}
		default:
			return fmt.Errorf("invalid delimiter")
		}
		return nil
	}
	if value() != nil {
		return false
	}
	_, err := dec.Token()
	return err == io.EOF
}

func normalizePlace(s string) string { return strings.Join(normalizedWords(s), " ") }

// Common commercial words and saint/day names are not reliable city evidence alone.
// This is a heuristic, not an exhaustive classification of Brazilian place names.
func genericMunicipalityName(name string) bool {
	switch name {
	case "central", "cidade", "domingo", "segunda", "terca", "quarta", "quinta", "sexta", "sabado", "bom jesus":
		return true
	}
	return strings.HasPrefix(name, "sao ") || strings.HasPrefix(name, "santa ") || strings.HasPrefix(name, "santo ")
}

func municipalityHasContext(input, name, uf string, candidates []destination.Candidate) bool {
	for _, c := range candidates {
		if normalizePlace(c.City) == name {
			return true
		}
	}
	text := " " + normalizePlace(input) + " "
	if strings.Contains(text, " em "+name+" ") || strings.Contains(text, " "+name+" "+normalizePlace(uf)+" ") {
		return true
	}
	// A comma or spaced dash before the place is explicit geographic context.
	for _, part := range strings.Split(strings.ReplaceAll(input, " - ", ","), ",")[1:] {
		segment := normalizePlace(part)
		if segment == name || strings.HasPrefix(segment, name+" ") {
			return true
		}
	}
	return false
}

// Reject uncertain or conflicting matches rather than sending to another city.
func constrainMunicipality(input string, candidates []destination.Candidate) ([]destination.Candidate, bool, error) {
	list, err := municipalityIndex()
	if err != nil {
		return nil, false, err
	}
	text := " " + normalizePlace(input) + " "
	names := map[string][]municipality{}
	for _, m := range list {
		n := normalizePlace(m.Name)
		if n != "" && strings.Contains(text, " "+n+" ") {
			if genericMunicipalityName(n) && !municipalityHasContext(input, n, m.UF, candidates) {
				continue
			}
			names[n] = append(names[n], m)
		}
	}
	// Longer names subsume shorter matching municipality names.
	for n := range names {
		for longer := range names {
			if n != longer && strings.Contains(" "+longer+" ", " "+n+" ") {
				delete(names, n)
				break
			}
		}
	}
	if len(names) == 0 {
		return candidates, false, nil
	}
	if len(names) != 1 {
		return nil, true, errors.New("ambiguous municipalities")
	}
	var name string
	var matches []municipality
	for name, matches = range names {
		break
	}
	if len(matches) > 1 {
		var narrowed []municipality
		for _, m := range matches {
			if strings.Contains(text, " "+normalizePlace(m.UF)+" ") {
				narrowed = append(narrowed, m)
			}
		}
		matches = narrowed
	}
	if len(matches) != 1 {
		return nil, true, errors.New("ambiguous municipality; include state")
	}
	m := matches[0]
	// A state immediately following the city is an explicit constraint, even
	// when the municipality name is unique in the index.
	for _, segment := range strings.Split(strings.ReplaceAll(input, " - ", ","), ",") {
		segment = normalizePlace(segment)
		if strings.HasPrefix(segment, name+" ") {
			state := strings.TrimSpace(strings.TrimPrefix(segment, name+" "))
			for _, uf := range municipalityUFs {
				if stateMatches(state, uf) && uf != m.UF {
					return nil, true, errors.New("municipality state conflict")
				}
			}
		}
	}
	segments := strings.Split(strings.ReplaceAll(input, " - ", ","), ",")
	for i := 0; i+1 < len(segments); i++ {
		if normalizePlace(segments[i]) != name {
			continue
		}
		for _, uf := range municipalityUFs {
			if stateMatches(segments[i+1], uf) && uf != m.UF {
				return nil, true, errors.New("municipality state conflict")
			}
		}
	}
	filtered := make([]destination.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if normalizePlace(c.City) != name {
			continue
		}
		if c.State != "" && !stateMatches(c.State, m.UF) {
			continue
		}
		// If state was required to disambiguate a homonym, absent state cannot prove a match.
		if len(names[name]) > 1 && c.State == "" {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return nil, true, errors.New("municipality not confirmed; clarify destination")
	}
	return filtered, true, nil
}

func stateMatches(state, uf string) bool {
	states := map[string]string{"AC": "acre", "AL": "alagoas", "AP": "amapa", "AM": "amazonas", "BA": "bahia", "CE": "ceara", "DF": "distrito federal", "ES": "espirito santo", "GO": "goias", "MA": "maranhao", "MT": "mato grosso", "MS": "mato grosso do sul", "MG": "minas gerais", "PA": "para", "PB": "paraiba", "PR": "parana", "PE": "pernambuco", "PI": "piaui", "RJ": "rio de janeiro", "RN": "rio grande do norte", "RS": "rio grande do sul", "RO": "rondonia", "RR": "roraima", "SC": "santa catarina", "SP": "sao paulo", "SE": "sergipe", "TO": "tocantins"}
	s := normalizePlace(state)
	full, valid := states[strings.ToUpper(uf)]
	return valid && (s == normalizePlace(uf) || s == full)
}
