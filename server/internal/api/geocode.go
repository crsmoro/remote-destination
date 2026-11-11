package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"waze-remoto/internal/destination"
)

type principalKey struct{}

type selection struct {
	Revision   int64                   `json:"r"`
	Expires    int64                   `json:"e"`
	Audience   string                  `json:"a"`
	Candidates []destination.Candidate `json:"c"`
}

func (a *server) selectionCipher() (cipher.AEAD, error) {
	a.tokenOnce.Do(func() { _, a.tokenErr = rand.Read(a.tokenKey[:]) })
	if a.tokenErr != nil {
		return nil, a.tokenErr
	}
	block, err := aes.NewCipher(a.tokenKey[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (a *server) geocodeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	audience := "web"
	if fields := strings.Fields(r.Header.Get("Authorization")); len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		audience = "agent"
	}
	auto := r.URL.Path == "/api/geocode/auto"
	if r.URL.Path == "/api/geocode" || auto {
		var in struct {
			Input string `json:"input"`
			City  string `json:"city"`
		}
		if !read(w, r, &in) {
			return
		}
		input := strings.TrimSpace(in.Input)
		if input == "" || !utf8.ValidString(input) || utf8.RuneCountInString(input) > 4096 {
			http.Error(w, "invalid address", 422)
			return
		}
		originalInput := input
		lower := strings.ToLower(input)
		if !strings.Contains(input, "://") && (strings.Contains(lower, "maps.google.") || strings.Contains(lower, "google.com/maps") || strings.Contains(lower, "maps.app.goo.gl") || strings.Contains(lower, "goo.gl/maps")) {
			http.Error(w, "invalid destination", 422)
			return
		}
		var revision int64
		var err error
		principal, key := "", ""
		if auto && len(r.Header.Values("Idempotency-Key")) > 0 {
			values := r.Header.Values("Idempotency-Key")
			if len(values) != 1 || len(values[0]) < 1 || len(values[0]) > 128 || strings.TrimSpace(values[0]) != values[0] || strings.ContainsAny(values[0], ",\r\n \t") {
				http.Error(w, "invalid idempotency key", 400)
				return
			}
			principal = "web" // Web uses the single upstream-authenticated account.
			if authenticated, ok := r.Context().Value(principalKey{}).(string); ok {
				principal = authenticated
			}
			kh := sha256.Sum256([]byte(values[0]))
			key = hex.EncodeToString(kh[:])
			canonical, _ := json.Marshal(in)
			bh := sha256.Sum256(canonical)
			var saved []byte
			revision, saved, err = a.s.BeginIdempotent(principal, key, hex.EncodeToString(bh[:]))
			if err != nil {
				fail(w, err)
				return
			}
			if saved != nil {
				var historical map[string]any
				if err := json.Unmarshal(saved, &historical); err != nil || historical == nil {
					http.Error(w, "invalid stored response", http.StatusInternalServerError)
					return
				}
				historical["replayed"] = true
				respond(w, 201, historical)
				return
			}
		} else {
			revision, err = a.s.BeginSubmit()
		}
		commitStarted := false
		if key != "" && err == nil {
			defer func() {
				if !commitStarted {
					_ = a.s.ReleaseIdempotent(principal, key, revision)
				}
			}()
		}
		confirm := func(uid, uri, display string, result map[string]any) error {
			if key == "" {
				return a.s.ConfirmIfCurrent(revision, uid, uri, display)
			}
			data, e := json.Marshal(result)
			if e != nil {
				return e
			}
			data = append(data, '\n')
			commitStarted = true // a database commit error may have an unknown outcome
			return a.s.ConfirmIdempotent(revision, uid, uri, display, principal, key, data)
		}
		if err != nil {
			fail(w, err)
			return
		}
		if auto && strings.Contains(strings.ToLower(input), "waze.com/") {
			uri, warning, normalizeErr := destination.Normalize(input)
			if normalizeErr == nil && warning == "" && destination.SafeCoordinateLink(input, uri) {
				uid, e := id()
				if e != nil {
					fail(w, e)
					return
				}
				result := map[string]any{"id": uid, "state": "pending", "replayed": false, "uri": uri, "display": input, "reference": "coordinates", "warning": nil}
				if e = confirm(uid, uri, input, result); e != nil {
					fail(w, e)
					return
				}
				respond(w, 201, result)
				return
			}
		}
		lookup := a.geocode
		var candidates []destination.Candidate
		if strings.Contains(input, "://") || strings.HasPrefix(input, "//") {
			u, parseErr := url.Parse(input)
			if parseErr == nil && (u.Hostname() == "waze.com" || u.Hostname() == "www.waze.com" || u.Hostname() == "ul.waze.com") {
				q, e := url.ParseQuery(u.RawQuery)
				valid := e == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && u.Fragment == "" && u.Path == "/ul" && len(q["q"]) == 1
				for k, v := range q {
					if len(v) != 1 || (k != "q" && k != "navigate" && k != "overview") || ((k == "navigate" || k == "overview") && v[0] != "yes" && v[0] != "no") {
						valid = false
					}
				}
				input = strings.TrimSpace(q.Get("q"))
				if !valid || input == "" || !utf8.ValidString(input) || strings.Contains(input, "://") || strings.HasPrefix(input, "//") {
					http.Error(w, "invalid destination", 422)
					return
				}
			} else {
				if parseErr != nil || (u.Hostname() != "www.google.com" && u.Hostname() != "maps.google.com" && u.Hostname() != "maps.app.goo.gl" && u.Hostname() != "goo.gl") {
					http.Error(w, "invalid destination", 422)
					return
				}
				uri, warning, normalizeErr := destination.Normalize(input)
				if normalizeErr != nil || warning != "" {
					http.Error(w, "Google link cannot be resolved", 422)
					return
				}
				parsed, _ := url.Parse(uri)
				if ll := parsed.Query().Get("ll"); ll != "" {
					parts := strings.Split(ll, ",")
					if len(parts) != 2 {
						http.Error(w, "invalid destination", 422)
						return
					}
					lat, e1 := strconv.ParseFloat(parts[0], 64)
					lon, e2 := strconv.ParseFloat(parts[1], 64)
					if e1 != nil || e2 != nil {
						http.Error(w, "invalid destination", 422)
						return
					}
					candidates = []destination.Candidate{{Label: "Coordenadas do Google Maps: " + ll, Latitude: lat, Longitude: lon}}
				} else {
					input = parsed.Query().Get("q")
					if input == "" || strings.Contains(input, "://") || strings.HasPrefix(input, "//") {
						http.Error(w, "invalid address", 422)
						return
					}
				}
			}
		}
		searchInput := input
		city := strings.TrimSpace(in.City)
		if !utf8.ValidString(city) || utf8.RuneCountInString(city) > 120 || strings.Contains(city, "://") || strings.HasPrefix(city, "//") {
			http.Error(w, "invalid city", 422)
			return
		}
		reference := "coordinates"
		if candidates == nil {
			if lookup == nil {
				http.Error(w, "geocoding unavailable", http.StatusBadGateway)
				return
			}
			if strings.EqualFold(strings.TrimSpace(os.Getenv("WAZE_GEOCODING_PROVIDER")), "google") && !strings.Contains(originalInput, "://") && !strings.HasPrefix(originalInput, "//") {
				input = in.Input
			}
			if city != "" {
				input += ", " + city
				reference = "explicit_city"
			} else {
				reference = "provider"
			}
			candidates, err = lookup(r.Context(), input)

		}
		if err != nil {
			if errors.Is(err, destination.ErrGoogleLimit) {
				http.Error(w, "google geocoding request limit reached", 429)
				return
			}
			http.Error(w, "geocoding unavailable", 502)
			return
		}
		if len(candidates) == 0 {
			if auto {
				http.Error(w, "no destinations found", 422)
				return
			}
			respond(w, 200, map[string]any{"candidates": []destination.Candidate{}, "reference": reference})
			return
		}
		if len(candidates) > 20 || (len(candidates) > 5 && !(reference != "coordinates" && (candidates[0].GoogleResult || candidates[0].GoogleName != ""))) {
			http.Error(w, "invalid geocoding response", 502)
			return
		}
		for i := range candidates {
			if !utf8.ValidString(candidates[i].City) || utf8.RuneCountInString(candidates[i].City) > 120 {
				candidates[i].City = ""
			}
			if !utf8.ValidString(candidates[i].State) || utf8.RuneCountInString(candidates[i].State) > 120 {
				candidates[i].State = ""
			}
			label := candidates[i].Label
			if !utf8.ValidString(label) || strings.TrimSpace(label) == "" {
				http.Error(w, "invalid geocoding response", 502)
				return
			}
			if len(label) > 128 && !(auto && (candidates[i].GoogleResult || candidates[i].GoogleName != "")) {
				prefix, suffix := label[:60], label[len(label)-65:]
				for !utf8.ValidString(prefix) {
					prefix = prefix[:len(prefix)-1]
				}
				for !utf8.ValidString(suffix) {
					suffix = suffix[1:]
				}
				candidates[i].Label = prefix + "..." + suffix
			}
			if math.IsNaN(candidates[i].Latitude) || math.IsNaN(candidates[i].Longitude) || math.Abs(candidates[i].Latitude) > 90 || math.Abs(candidates[i].Longitude) > 180 {
				http.Error(w, "invalid geocoding response", 502)
				return
			}
		}
		googleResult := reference != "coordinates" && (candidates[0].GoogleResult || candidates[0].GoogleName != "")
		if auto && reference != "coordinates" && !googleResult {
			if city != "" {
				municipalities, err := municipalityIndex()
				if err != nil || len(municipalities) == 0 {
					http.Error(w, "índice municipal indisponível", 422)
					return
				}
			}
			{
				var matched bool
				candidates, matched, err = constrainMunicipality(searchInput, candidates)
				if err != nil {
					http.Error(w, "cidade ambígua ou incompatível; esclareça o destino", 422)
					return
				}
				if matched && city == "" {
					lat, lon, _ := positionReference()
					rankCandidates(candidates, lat, lon)
					reference = "possible_text_city"
				}
			}
			if city != "" {
				filtered := candidates[:0]
				for _, candidate := range candidates {
					if strings.Join(normalizedWords(candidate.City), " ") == strings.Join(normalizedWords(city), " ") && len(normalizedWords(city)) > 0 {
						filtered = append(filtered, candidate)
					}
				}
				candidates = filtered
			}
			filtered := candidates[:0]
			for _, candidate := range candidates {
				if plausibleAutomaticCandidate(searchInput, candidate) {
					filtered = append(filtered, candidate)
				}
			}
			candidates = filtered
			if len(candidates) == 0 {
				http.Error(w, "no compatible destinations found", 422)
				return
			}
			if reference == "explicit_city" {
				lat, lon, _ := positionReference()
				rankCandidates(candidates, lat, lon)
			}
		}
		if googleResult && auto && reference != "coordinates" {
			if city != "" {
				prioritizeGoogleCity(candidates, city)
			} else if inferredGoogleCity(candidates, searchInput) != "" {
				prioritizeGoogleCity(candidates, inferredGoogleCity(candidates, searchInput))
				reference = "possible_text_city"
			} else {
				lat, lon, ref := positionReference()
				rankCandidates(candidates, lat, lon)
				reference = ref
			}
		} else if reference == "provider" {
			if auto {
				lat, lon, _ := positionReference()
				rankCandidates(candidates, lat, lon)
			}
			if prioritizePossibleCity(candidates, input) {
				reference = "possible_text_city"
			} else {
				lat, lon, ref := positionReference()
				rankCandidates(candidates, lat, lon)
				reference = ref
			}
		}
		if auto {
			candidate := candidates[0]
			values := url.Values{"ll": {strconv.FormatFloat(candidate.Latitude, 'f', -1, 64) + "," + strconv.FormatFloat(candidate.Longitude, 'f', -1, 64)}, "navigate": {"yes"}}
			uri := "https://waze.com/ul?" + values.Encode()
			uid, e := id()
			if e != nil {
				fail(w, e)
				return
			}
			var warning any
			result := map[string]any{"id": uid, "state": "pending", "replayed": false, "uri": uri, "display": candidate.Label, "reference": reference, "warning": warning}
			if e = confirm(uid, uri, originalInput, result); e != nil {
				fail(w, e)
				return
			}
			respond(w, 201, result)
			return
		}
		if len(candidates) > 5 {
			candidates = candidates[:5]
		}
		c, err := a.selectionCipher()
		if err != nil {
			fail(w, err)
			return
		}
		nonce := make([]byte, c.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			fail(w, err)
			return
		}
		data, err := json.Marshal(selection{revision, time.Now().Add(5 * time.Minute).Unix(), audience, candidates})
		if err != nil {
			fail(w, err)
			return
		}
		token := base64.RawURLEncoding.EncodeToString(c.Seal(nonce, nonce, data, nil))
		if len(token) > 8000 {
			http.Error(w, "invalid geocoding response", 502)
			return
		}
		respond(w, 200, map[string]any{"token": token, "candidates": candidates, "reference": reference})
		return
	}
	var in struct {
		Token string `json:"token"`
		Index *int   `json:"index"`
	}
	if !read(w, r, &in) {
		return
	}
	if in.Index == nil || len(in.Token) > 8000 {
		http.Error(w, "invalid selection", 422)
		return
	}
	c, err := a.selectionCipher()
	if err != nil {
		fail(w, err)
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(in.Token)
	if err != nil || len(raw) < c.NonceSize() {
		http.Error(w, "invalid selection", 422)
		return
	}
	data, err := c.Open(nil, raw[:c.NonceSize()], raw[c.NonceSize():], nil)
	var sel selection
	if err != nil || json.Unmarshal(data, &sel) != nil || sel.Audience != audience || time.Now().Unix() >= sel.Expires || *in.Index < 0 || *in.Index >= len(sel.Candidates) {
		http.Error(w, "invalid selection", 422)
		return
	}
	candidate := sel.Candidates[*in.Index]
	if candidate.Label == "" || math.IsNaN(candidate.Latitude) || math.IsNaN(candidate.Longitude) || math.Abs(candidate.Latitude) > 90 || math.Abs(candidate.Longitude) > 180 {
		http.Error(w, "invalid selection", 422)
		return
	}
	values := url.Values{"ll": {strconv.FormatFloat(candidate.Latitude, 'f', -1, 64) + "," + strconv.FormatFloat(candidate.Longitude, 'f', -1, 64)}, "navigate": {"yes"}}
	uri := "https://waze.com/ul?" + values.Encode()
	uid, err := id()
	if err != nil {
		fail(w, err)
		return
	}
	if err = a.s.ConfirmIfCurrent(sel.Revision, uid, uri, candidate.Label); err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, map[string]any{"id": uid, "state": "pending", "uri": uri, "warning": nil})
}
