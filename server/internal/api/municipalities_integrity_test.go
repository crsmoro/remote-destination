package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMunicipalityRejectsValidButTruncatedJSON(t *testing.T) {
	path := t.TempDir() + "/index.json"
	if err := os.WriteFile(path, []byte(`[{"id":4106902,"nome":"Curitiba","uf":"PR"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", path)
	if _, err := municipalityIndex(); err == nil {
		t.Fatal("accepted incomplete valid JSON index")
	}
}

// Build realistic-sized synthetic snapshots without importing official data.
func syntheticSnapshot(extra []municipality) []byte {
	rows := append([]municipality{}, extra...)
	used := map[int]bool{}
	for _, m := range rows {
		used[m.ID] = true
	}
	for prefix, uf := range municipalityUFs {
		id := prefix*100000 + 99999
		if !used[id] {
			rows = append(rows, municipality{ID: id, Name: fmt.Sprintf("Teste %d", id), UF: uf})
		}
	}
	for prefix, uf := range municipalityUFs {
		for i := 0; i < 200 && len(rows) < 5000; i++ {
			id := prefix*100000 + i
			if !used[id] {
				rows = append(rows, municipality{ID: id, Name: fmt.Sprintf("Teste %d", id), UF: uf})
			}
		}
	}
	raw, _ := json.Marshal(rows)
	sum := sha256.Sum256(raw)
	return []byte(fmt.Sprintf(`{"version":1,"count":%d,"sha256":"%x","municipalities":%s}`, len(rows), sum, raw))
}
func TestMunicipalitySnapshotIntegrity(t *testing.T) {
	good := syntheticSnapshot(nil)
	if _, err := validateMunicipalitySnapshot(good); err != nil {
		t.Fatal(err)
	}
	var env struct {
		Version int            `json:"version"`
		Count   int            `json:"count"`
		SHA     string         `json:"sha256"`
		Rows    []municipality `json:"municipalities"`
	}
	if err := json.Unmarshal(good, &env); err != nil {
		t.Fatal(err)
	}
	wrap := func(rows []municipality) []byte {
		raw, _ := json.Marshal(rows)
		sum := sha256.Sum256(raw)
		return []byte(fmt.Sprintf(`{"version":1,"count":%d,"sha256":"%x","municipalities":%s}`, len(rows), sum, raw))
	}
	duplicate := append([]municipality{}, env.Rows...)
	duplicate[1] = duplicate[0]
	invalidUF := append([]municipality{}, env.Rows...)
	invalidUF[0].UF = "ZZ"
	control := append([]municipality{}, env.Rows...)
	control[0].Name = "teste\n"
	cases := map[string][]byte{
		"changed":        bytes.Replace(good, []byte("Teste"), []byte("Xeste"), 1),
		"removed-field":  bytes.Replace(good, []byte(`,"nome":`), []byte(`,"none":`), 1),
		"removed-entry":  bytes.Replace(good, []byte(fmt.Sprintf(`%s,`, mustJSON(env.Rows[0]))), nil, 1),
		"empty":          []byte(`{"version":1,"count":0,"sha256":"","municipalities":[]}`),
		"short-rehashed": wrap(env.Rows[:100]),
		"duplicate":      wrap(duplicate), "uf": wrap(invalidUF), "control": wrap(control),
		"duplicate-key": bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		"unknown":       bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1,"x":1`), 1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := validateMunicipalitySnapshot(data); err == nil {
				t.Fatal("corruption accepted")
			}
		})
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", t.TempDir()+"/missing")
	if ValidateMunicipalityIndex() == nil {
		t.Fatal("missing file accepted")
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", "")
	if ValidateMunicipalityIndex() != nil {
		t.Fatal("optional index rejected")
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func TestPythonGeneratedSnapshotLoads(t *testing.T) {
	out := filepath.Join(t.TempDir(), "municipios.json")
	cmd := exec.Command("python3", "-c", `import importlib.util, pathlib, sys
root=pathlib.Path(sys.argv[1]); spec=importlib.util.spec_from_file_location("m",root/"tests/test_municipios_provisionamento.py"); m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
pathlib.Path(sys.argv[2]).write_bytes(m.gen.convert(m.json.dumps(m.fixture(), ensure_ascii=False).encode()))`, "../../..", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generator: %v: %s", err, output)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", out)
	if err := ValidateMunicipalityIndex(); err != nil {
		t.Fatal(err)
	}
}
