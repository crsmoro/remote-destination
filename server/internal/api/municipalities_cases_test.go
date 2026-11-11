package api

import (
	"os"
	"testing"
	"waze-remoto/internal/destination"
)

func TestMunicipalityConservativeCases(t *testing.T) {
	file := t.TempDir() + "/index.json"
	data := syntheticSnapshot([]municipality{{4106902, "Curitiba", "PR"}, {4209102, "Joinville", "SC"}, {2202000, "Bom Jesus", "PI"}, {4302000, "Bom Jesus", "RS"}, {3550308, "São Paulo", "SP"}})
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", file)
	cases := []struct {
		name, input   string
		candidates    []destination.Candidate
		count         int
		matched, fail bool
	}{
		{"accent", "loja em SAO PAULO", []destination.Candidate{{City: "São Paulo", State: "São Paulo"}}, 1, true, false},
		{"whole words", "curitibano", []destination.Candidate{{City: "Joinville"}}, 1, false, false},
		{"two places", "Curitiba Joinville", []destination.Candidate{{City: "Joinville"}}, 0, true, true},
		{"homonym", "Bom Jesus", []destination.Candidate{{City: "Bom Jesus", State: "PI"}}, 0, true, true},
		{"homonym state", "Bom Jesus PI", []destination.Candidate{{City: "Bom Jesus", State: "Piauí"}, {City: "Bom Jesus", State: "RS"}}, 1, true, false},
		{"homonym missing state", "Bom Jesus PI", []destination.Candidate{{City: "Bom Jesus"}}, 0, true, true},
		{"business false positive", "Empresa Bom Jesus", []destination.Candidate{{City: "Joinville"}}, 1, false, false},
		{"foreign not detected", "Paris", []destination.Candidate{{City: "Paris"}}, 1, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, m, e := constrainMunicipality(tc.input, tc.candidates)
			if len(c) != tc.count || m != tc.matched || (e != nil) != tc.fail {
				t.Fatalf("c=%+v matched=%v err=%v", c, m, e)
			}
		})
	}
}
