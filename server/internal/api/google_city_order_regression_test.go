package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleCityTextPreservesProviderOrder(t *testing.T) {
	for _, tc := range []struct{ input, display, reference string }{
		{"Empresa, Joinville", "primeiro", "possible_text_city"},
		{"Escola Joinville", "perto", "joinville"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Setenv("WAZE_POSITION_FILE", "")
			s, e := store.Open(t.TempDir() + "/db")
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
				return []destination.Candidate{
					{GoogleResult: true, Label: "primeiro", City: "Joinville", Latitude: 0, Longitude: 0},
					{GoogleResult: true, GoogleName: "Escola Joinville", Label: "perto", Latitude: -26.3045, Longitude: -48.8487},
				}, nil
			}
			body, _ := json.Marshal(map[string]string{"input": tc.input})
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(string(body))))
			var out struct {
				Display   string `json:"display"`
				Reference string `json:"reference"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if w.Code != 201 || out.Display != tc.display || out.Reference != tc.reference {
				t.Fatalf("status=%d result=%+v", w.Code, out)
			}
		})
	}
}
