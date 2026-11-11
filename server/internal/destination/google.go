package destination

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

func searchGoogle(ctx context.Context, client *http.Client, key, input string) ([]Candidate, error) {
	return searchGoogleWithPlaces(ctx, client, key, input, nil)
}

func searchGoogleWithPlaces(ctx context.Context, client *http.Client, key, input string, allowPlaces func() error) ([]Candidate, error) {
	results, err := searchGoogleDestinations(ctx, client, key, input)
	if err != nil || len(results) != 0 {
		return results, err
	}
	if allowPlaces != nil {
		if err := allowPlaces(); err != nil {
			return nil, err
		}
	}
	return searchPlaces(ctx, client, key, input)
}

func searchGoogleDestinations(ctx context.Context, client *http.Client, key, input string) ([]Candidate, error) {
	if strings.TrimSpace(input) == "" || !utf8.ValidString(input) || utf8.RuneCountInString(input) > 4096 || strings.Contains(input, "://") || strings.HasPrefix(strings.TrimSpace(input), "//") || strings.TrimSpace(key) == "" {
		return nil, errors.New("invalid geocoding request")
	}
	payload, _ := json.Marshal(map[string]any{"addressQuery": map[string]string{"addressQuery": input}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://geocode.googleapis.com/v4/geocode/destinations", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("invalid geocoding request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", key)
	req.Header.Set("X-Goog-FieldMask", "destinations.primary.displayName,destinations.primary.formattedAddress,destinations.primary.location,destinations.primary.addressComponents,destinations.primary.postalAddress,destinations.primary.placeId")
	copyClient := *client
	copyClient.Timeout = 8 * time.Second
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := copyClient.Do(req)
	if err != nil {
		return nil, errors.New("geocoding unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("geocoding unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || len(body) >= 1<<20 || !utf8.Valid(body) {
		return nil, errors.New("invalid geocoding response")
	}
	var data struct {
		Status       *string         `json:"status"`
		Error        json.RawMessage `json:"error"`
		Results      json.RawMessage `json:"results"`
		Destinations []struct {
			Primary *struct {
				DisplayName struct {
					Text string `json:"text"`
				} `json:"displayName"`
				FormattedAddress string `json:"formattedAddress"`
				PostalAddress    struct {
					Locality string `json:"locality"`
				} `json:"postalAddress"`
				Location *struct {
					Latitude  *float64 `json:"latitude"`
					Longitude *float64 `json:"longitude"`
				} `json:"location"`
				AddressComponents []struct {
					LongText string   `json:"longText"`
					Types    []string `json:"types"`
				} `json:"addressComponents"`
			} `json:"primary"`
		} `json:"destinations"`
	}
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(body, &data) != nil || data.Status != nil || data.Error != nil || data.Results != nil {
		return nil, errors.New("invalid geocoding response")
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil {
		return nil, errors.New("invalid geocoding response")
	}
	for name, value := range envelope {
		if name != "destinations" || string(value) == "null" {
			return nil, errors.New("invalid geocoding response")
		}
	}
	var out []Candidate
	for _, item := range data.Destinations {
		r := item.Primary
		if r == nil || r.Location == nil || r.Location.Latitude == nil || r.Location.Longitude == nil || !utf8.ValidString(r.DisplayName.Text) || !utf8.ValidString(r.FormattedAddress) {
			continue
		}
		lat, lon := *r.Location.Latitude, *r.Location.Longitude
		if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
			continue
		}
		city, admin := "", ""
		for _, component := range r.AddressComponents {
			for _, typ := range component.Types {
				if typ == "locality" {
					city = component.LongText
				}
				if typ == "administrative_area_level_2" {
					admin = component.LongText
				}
			}
		}
		if city == "" {
			city = r.PostalAddress.Locality
		}
		if city == "" {
			city = admin
		}
		label := r.FormattedAddress
		if strings.TrimSpace(label) == "" {
			label = r.DisplayName.Text
		} else if strings.TrimSpace(r.DisplayName.Text) != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(r.DisplayName.Text)) {
			label = r.DisplayName.Text + ", " + label
		}
		if strings.TrimSpace(label) == "" {
			label = input
		}
		out = append(out, Candidate{GoogleResult: true, GoogleName: r.DisplayName.Text, GoogleAddress: r.FormattedAddress, Label: label, City: city, Latitude: lat, Longitude: lon})

	}
	if len(data.Destinations) > 0 && len(out) == 0 {
		return nil, errors.New("invalid geocoding response")
	}
	return out, nil
}
