package api

import (
	"testing"
	"waze-remoto/internal/destination"
)

func TestPossibleCityBoundariesAndAmbiguity(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  bool
		first string
	}{
		{"padaria SAO PAULO", true, "sp"},
		{"padaria saopaulo", false, "joinville"},
		{"padaria Sao Paulo Joinville", false, "joinville"},
		{"padaria Curitiba", false, "joinville"},
		{"padaria São Paulo", true, "sp"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			c := []destination.Candidate{{Label: "joinville", City: "Joinville"}, {Label: "sp", City: "São Paulo"}, {Label: "sem cidade"}}
			got := prioritizePossibleCity(c, tt.input)
			if got != tt.want || c[0].Label != tt.first || len(c) != 3 {
				t.Fatalf("got %v %v", got, c)
			}
		})
	}
}
