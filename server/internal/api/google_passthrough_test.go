package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleAutomaticSelectionAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		code             int
		candidates       []destination.Candidate
	}{
		{"different street", `{"input":"Rua Exemplo, 123"}`, "-28%2C-50", 201, []destination.Candidate{{GoogleName: "Avenida Outra", GoogleAddress: "Avenida Outra, 7", Label: "Avenida Outra, 7", Latitude: -28, Longitude: -50}}},
		{"nearest without city", `{"input":"loja"}`, "-26.3045%2C-48.8487", 201, []destination.Candidate{{GoogleName: "Um", Label: "Um", City: "Outra", Latitude: -28, Longitude: -50}, {GoogleName: "Dois", Label: "Dois", City: "Joinville", Latitude: -26.3045, Longitude: -48.8487}}},
		{"city in text prioritizes matching city", `{"input":"loja, Joinville"}`, "-26.3045%2C-48.8487", 201, []destination.Candidate{{GoogleName: "Um", Label: "Um", City: "Outra", Latitude: -28, Longitude: -50}, {GoogleName: "Dois", Label: "Dois", City: "Joinville", Latitude: -26.3045, Longitude: -48.8487}}},
		{"city field prioritizes matching city", `{"input":"loja","city":"Joinville"}`, "-26.3045%2C-48.8487", 201, []destination.Candidate{{GoogleName: "Um", Label: "Um", City: "Outra", Latitude: -28, Longitude: -50}, {GoogleName: "Dois", Label: "Dois", City: "Joinville", Latitude: -26.3045, Longitude: -48.8487}}},
		{"no result", `{"input":"loja"}`, "", 422, nil},
		{"invalid coordinate", `{"input":"loja"}`, "", 502, []destination.Candidate{{GoogleName: "Um", Label: "Um", Latitude: 91, Longitude: 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WAZE_POSITION_FILE", "")
			s, e := store.Open(t.TempDir() + "/db")
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(_ context.Context, _ string) ([]destination.Candidate, error) { return tc.candidates, nil }
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(tc.body)))
			if w.Code != tc.code {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			p, e := s.Pending()
			if e != nil {
				t.Fatal(e)
			}
			if tc.code == 201 && (p == nil || !strings.Contains(p.URI, tc.want)) {
				t.Fatalf("pending=%+v", p)
			}
			if tc.code != 201 && p != nil {
				t.Fatalf("unexpected pending=%+v", p)
			}
		})
	}
}

func TestGoogleAutomaticTrustsProviderResult(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		if q != "cei" {
			t.Errorf("query changed: %q", q)
		}
		return []destination.Candidate{{GoogleName: "Centro de Educação Infantil", GoogleAddress: "Avenida Exemplo, 123", Label: "Centro de Educação Infantil, Avenida Exemplo, 123", City: "Joinville", Latitude: -26.3, Longitude: -48.8}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"cei"}`)))
	if w.Code != 201 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	p, e := s.Pending()
	if e != nil || p == nil || !strings.Contains(p.URI, "-26.3%2C-48.8") {
		t.Fatalf("pending=%+v err=%v", p, e)
	}
}
