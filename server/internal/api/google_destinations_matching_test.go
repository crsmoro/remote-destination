package api

import (
	"testing"
	"waze-remoto/internal/destination"
)

func TestGoogleDestinationNameSafety(t *testing.T) {
	for _, tc := range []struct {
		query, name, address, city string
		want                       bool
	}{
		{"zx armazem", "ZX ARMAZEM", "Rua Nebulosa, 10, Cidade Ficticia", "Cidade Ficticia", true},
		{"zx armazem", "Armazem", "Rua ZX, 10, Cidade Ficticia", "Cidade Ficticia", false},
		{"zx armazem", "Loja diversa", "Rua ZX Armazem, 10, Cidade Ficticia", "Cidade Ficticia", false},
		{"zx armazem Cidade Ficticia", "Loja diversa", "Rua Nebulosa, 10, Cidade Ficticia", "Cidade Ficticia", false},
		{"Rua Nebulosa, 10", "ZX ARMAZEM", "Rua Nebulosa, 10, Cidade Ficticia", "Cidade Ficticia", true},
		{"Rua Nebulosa, 10", "ZX ARMAZEM", "Rua Nebulosa, 11, Cidade Ficticia", "Cidade Ficticia", false},
	} {
		c := destination.Candidate{GoogleName: tc.name, GoogleAddress: tc.address, Label: tc.name + ", " + tc.address, City: tc.city}
		if got := plausibleAutomaticCandidate(tc.query, c); got != tc.want {
			t.Errorf("%q / %q / %q: got %v", tc.query, tc.name, tc.address, got)
		}
	}
}
