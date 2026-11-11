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

func searchPlaces(ctx context.Context, client *http.Client, key, input string) ([]Candidate, error) {
	payload, _ := json.Marshal(map[string]string{"textQuery": input})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://places.googleapis.com/v1/places:searchText", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("invalid geocoding request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", key)
	req.Header.Set("X-Goog-FieldMask", "places.displayName,places.formattedAddress,places.location,places.addressComponents")
	c := *client
	c.Timeout = 8 * time.Second
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(req)
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
		Error  json.RawMessage `json:"error"`
		Places []struct {
			DisplayName struct {
				Text string `json:"text"`
			} `json:"displayName"`
			FormattedAddress  string `json:"formattedAddress"`
			AddressComponents []struct {
				LongText  string   `json:"longText"`
				ShortText string   `json:"shortText"`
				Types     []string `json:"types"`
			} `json:"addressComponents"`
			Location *struct {
				Latitude  *float64 `json:"latitude"`
				Longitude *float64 `json:"longitude"`
			} `json:"location"`
		} `json:"places"`
	}
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(body, &data) != nil || data.Error != nil {
		return nil, errors.New("invalid geocoding response")
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil {
		return nil, errors.New("invalid geocoding response")
	}
	for name, value := range envelope {
		if name != "places" || string(value) == "null" {
			return nil, errors.New("invalid geocoding response")
		}
	}
	var out []Candidate
	for _, p := range data.Places {
		if p.Location == nil || p.Location.Latitude == nil || p.Location.Longitude == nil {
			continue
		}
		lat, lon := *p.Location.Latitude, *p.Location.Longitude
		if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
			continue
		}
		label := p.FormattedAddress
		if strings.TrimSpace(label) == "" {
			label = p.DisplayName.Text
		} else if strings.TrimSpace(p.DisplayName.Text) != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(p.DisplayName.Text)) {
			label = p.DisplayName.Text + ", " + label
		}
		if strings.TrimSpace(label) == "" {
			label = input
		}
		city, admin, state := "", "", ""
		for _, component := range p.AddressComponents {
			for _, typ := range component.Types {
				switch typ {
				case "locality":
					city = component.LongText
				case "administrative_area_level_2":
					admin = component.LongText
				case "administrative_area_level_1":
					state = component.ShortText
					if state == "" {
						state = component.LongText
					}
				}
			}
		}
		if city == "" {
			city = admin
		}
		out = append(out, Candidate{GoogleResult: true, GoogleName: p.DisplayName.Text, GoogleAddress: p.FormattedAddress, Label: label, City: city, State: state, Latitude: lat, Longitude: lon})
	}
	if len(data.Places) > 0 && len(out) == 0 {
		return nil, errors.New("invalid geocoding response")
	}
	return out, nil
}
