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

func TestGoogleCityPriorityCases(t *testing.T) {
	cases := []struct{ input, city, want, query string }{
		{"empresa ficticia vila imaginária", "", "cidade", "empresa ficticia vila imaginária"},
		{"empresa ficticia", "Vila Imaginária", "cidade", "empresa ficticia, Vila Imaginária"},
		{"empresa ficticia", "Desconhecida", "perto", "empresa ficticia, Desconhecida"},
		{"empresa ficticia", "", "perto", "empresa ficticia"},
		{"Escola Vila Imaginária", "", "perto", "Escola Vila Imaginária"},
		{"EMPRESA FICTICIA VILA IMAGINA\u0301RIA", "", "cidade", "EMPRESA FICTICIA VILA IMAGINA\u0301RIA"},
	}
	for _, tc := range cases {
		t.Run(tc.input+tc.city, func(t *testing.T) {
			t.Setenv("WAZE_POSITION_FILE", "")
			s, err := store.Open(t.TempDir() + "/db")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
				if q != tc.query {
					t.Errorf("query=%q want %q", q, tc.query)
				}
				return []destination.Candidate{
					{GoogleResult: true, GoogleName: "Escola Vila Imaginária", Label: "perto", City: "Curitiba", Latitude: -26.3045, Longitude: -48.8487},
					{GoogleResult: true, Label: "cidade", City: "Vila Imaginária", Latitude: 0, Longitude: 0},
					{GoogleResult: true, Label: "outra", Latitude: 1, Longitude: 1},
				}, nil
			}
			body, _ := json.Marshal(map[string]string{"input": tc.input, "city": tc.city})
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(string(body))))
			var out struct {
				Display string `json:"display"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if w.Code != 201 || out.Display != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGoogleCityStablePartition(t *testing.T) {
	c := []destination.Candidate{{Label: "a"}, {Label: "b", City: "Vila Imaginária"}, {Label: "c", City: "vila imaginária"}, {Label: "d"}}
	prioritizeGoogleCity(c, "VILA IMAGINÁRIA")
	var labels []string
	for _, v := range c {
		labels = append(labels, v.Label)
	}
	if strings.Join(labels, ",") != "b,c,a,d" {
		t.Fatal(labels)
	}
}

func TestGoogleSuffixCityPriority(t *testing.T) {
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		return []destination.Candidate{
			{GoogleResult: true, Label: "perto", City: "Curitiba", Latitude: -26.3045, Longitude: -48.8487},
			{GoogleResult: true, Label: "cidade", City: "Vila Imaginária", Latitude: 0, Longitude: 0},
		}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"empresa ficticia vila imaginária"}`)))
	var out struct {
		Display string `json:"display"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 201 || out.Display != "cidade" {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
