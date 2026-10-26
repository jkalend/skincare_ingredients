package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jkalend/skincare_ingredients/internal/inci"
	"github.com/jkalend/skincare_ingredients/internal/store"
)

const annexHeader = `"File creation date: 26/10/2026"
"ANNEX II","Last update: 13/10/2026"
"TEST ANNEX"
"Substance identification"
"Reference Number","Chemical name / INN","CAS Number","EC Number",Regulation,"Other Directives/Regulations","SCCS opinions","Chemical/IUPAC Name","Identified INGREDIENTS or substances e.g.",CMR,"Update Date"
1388,"Octamethylcyclotetrasiloxane; D4","556-67-2","209-136-7","(EU) 2019/831","","Opinion on D4","Octamethylcyclotetrasiloxane; D4","CYCLOMETHICONE","Reprotoxic Cat. 2()","17/10/2010"
`

// Store creates a small on-disk registry and returns an opened store.
func Store(t testing.TB) *store.Store {
	t.Helper()
	opened, _, _ := Setup(t)
	return opened
}

// Setup is Store plus the data/local directories, for CLI tests.
func Setup(t testing.TB) (*store.Store, string, string) {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	localDir := filepath.Join(root, "local")
	cosing := filepath.Join(dataDir, "registry", "cosing")
	if err := os.MkdirAll(cosing, 0o755); err != nil {
		t.Fatal(err)
	}
	records := []inci.IngredientRecord{
		{INCIName: "AQUA", Functions: []string{"SOLVENT"}, Source: inci.SourceRef{Source: "test", Method: "fixture"}},
		{INCIName: "GLYCERIN", Functions: []string{"HUMECTANT", "SOLVENT"}, Source: inci.SourceRef{Source: "test", Method: "fixture"}},
		{INCIName: "NIACINAMIDE", Functions: []string{"SMOOTHING"}, Source: inci.SourceRef{Source: "test", Method: "fixture"}},
		{INCIName: "PARFUM", Functions: []string{"FRAGRANCE"}, Source: inci.SourceRef{Source: "test", Method: "fixture"}},
		{INCIName: "SODIUM BENZOATE", Functions: []string{"PRESERVATIVE"}, Source: inci.SourceRef{Source: "test", Method: "fixture"}},
	}
	writeJSON(t, filepath.Join(cosing, "ingredients.json"), records)
	manifest := inci.Manifest{
		SchemaVersion: 1,
		GeneratedBy:   "test",
		FetchedAt:     "2026-10-26T00:00:00Z",
		Annexes: map[string]inci.ManifestAnnex{
			"II":  {SourceURL: "test", FetchedAt: "2026-10-26T00:00:00Z", ContentSHA256: "abc", LastUpdate: "13/10/2026", DataRows: 1},
			"III": {SourceURL: "test", FetchedAt: "2026-10-26T00:00:00Z", ContentSHA256: "def", LastUpdate: "13/10/2026"},
			"IV":  {SourceURL: "test", FetchedAt: "2026-10-26T00:00:00Z", ContentSHA256: "ghi", LastUpdate: "13/10/2026"},
			"V":   {SourceURL: "test", FetchedAt: "2026-10-26T00:00:00Z", ContentSHA256: "jkl", LastUpdate: "13/10/2026"},
			"VI":  {SourceURL: "test", FetchedAt: "2026-10-26T00:00:00Z", ContentSHA256: "mno", LastUpdate: "13/10/2026"},
		},
		Mirror: inci.ManifestMirror{Source: "test", FetchedAt: "2026-10-26T00:00:00Z", GlossaryVersion: "EU 2025/1175"},
	}
	writeJSON(t, filepath.Join(cosing, "manifest.json"), manifest)
	for _, annex := range []string{"II", "III", "IV", "V", "VI"} {
		body := annexHeader
		if annex != "II" {
			body = `"File creation date: 26/10/2026"
"ANNEX ` + annex + `","Last update: 13/10/2026"
"TEST ANNEX"
"Substance identification"
"Reference Number","Chemical name / INN","CAS Number","EC Number",Regulation,"Other Directives/Regulations","SCCS opinions","Chemical/IUPAC Name","Identified INGREDIENTS or substances e.g.",CMR,"Update Date"
`
		}
		if err := os.WriteFile(filepath.Join(cosing, "annex_"+annex+".csv"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{filepath.Join(dataDir, "products"), filepath.Join(dataDir, "shelf"), filepath.Join(localDir, "shelf"), filepath.Join(localDir, "products")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	opened, err := store.Open(store.Options{DataDir: dataDir, LocalDir: localDir, Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	return opened, dataDir, localDir
}

func writeJSON(t testing.TB, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
