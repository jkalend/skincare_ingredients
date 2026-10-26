package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jkalend/skincare_ingredients/internal/inci"
)

func TestSnapshotLoads(t *testing.T) {
	dir := filepath.Join("..", "..", "data", "registry", "cosing")
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Skip("registry snapshot not present")
	}
	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Records) < 30000 {
		t.Fatalf("only %d ingredient records", len(registry.Records))
	}
	if len(registry.Annex) < 2300 {
		t.Fatalf("only %d annex entries", len(registry.Annex))
	}
	info := registry.Info()
	if info.CosIngAnnexes != "13/10/2026" || info.SHA256 == "" {
		t.Fatalf("registry info = %#v", info)
	}
	record, _ := registry.Resolver.IngredientRecord("Aqua")
	if record == nil || record.INCIName != "AQUA" {
		t.Fatalf("Aqua record = %#v", record)
	}
	var entry *inci.AnnexEntry
	for i := range registry.Annex {
		if registry.Annex[i].Annex == "II" && registry.Annex[i].Reference == "1388" {
			entry = &registry.Annex[i]
			break
		}
	}
	if entry == nil {
		t.Fatal("Annex II entry 1388 missing")
	}
	if len(entry.NormalisedNames) == 0 {
		t.Fatal("entry 1388 has no names")
	}
}
