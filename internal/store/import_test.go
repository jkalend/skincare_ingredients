package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportInventoryIsLocalValidatedAndIdempotent(t *testing.T) {
	root := t.TempDir()
	localDir := filepath.Join(root, "local")
	store, err := Open(Options{
		DataDir:  filepath.Join("..", "..", "data"),
		LocalDir: localDir,
		CacheDir: filepath.Join(root, "cache"),
		Offline:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "inventory.json")
	writeManifest := func(panel string) {
		t.Helper()
		manifest := `{
  "schema_version": 1,
  "shelf": "owned",
  "products": [{
    "product_key": "local:test-cream",
    "brand": "Test",
    "name": "Cream",
    "units_owned": 2,
    "inci": "Aqua, Glycerin, Niacinamide` + strings.TrimPrefix(panel, "Aqua, Glycerin, Niacinamide") + `",
    "panel_type": "water-based",
    "source": {
      "source": "manufacturer",
      "url": "https://manufacturer.example/cream",
      "fetched_at": "2026-10-26T14:00:00Z",
      "method": "manufacturer_page",
      "confidence": 0.8,
      "robots_allowed": true
    }
  }]
}`
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest("Aqua, Glycerin, Niacinamide")

	result, err := store.ImportInventory(manifestPath, "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Products != 1 || result.Units != 2 || !result.DryRun {
		t.Fatalf("dry-run result = %#v", result)
	}
	if _, err := os.Stat(localDir); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote into local directory: %v", err)
	}

	result, err = store.ImportInventory(manifestPath, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.DryRun || result.Sources != 1 || result.Ingredients != 3 {
		t.Fatalf("import result = %#v", result)
	}
	product := store.ProductByKey("local:test-cream")
	if product == nil || product.UnitsOwned != 2 || product.Trust != "unverified" || product.Source == nil {
		t.Fatalf("imported product = %#v", product)
	}
	if got := len(store.Shelf("owned").Items); got != 1 {
		t.Fatalf("shelf items = %d, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(localDir, "sources")); err != nil {
		t.Fatalf("private source observations not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "data", "products", "local_test-cream.json")); !os.IsNotExist(err) {
		t.Fatalf("private import wrote under data/products: %v", err)
	}
	if _, err := store.ImportInventory(manifestPath, "", false, false); err != nil {
		t.Fatalf("unchanged import should be idempotent: %v", err)
	}

	before, err := os.ReadFile(filepath.Join(localDir, "products", "local_test-cream.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeManifest("Aqua, Glycerin, Niacinamide, Panthenol")
	if _, err := store.ImportInventory(manifestPath, "", false, false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("changed import error = %v, want --force guidance", err)
	}
	after, err := os.ReadFile(filepath.Join(localDir, "products", "local_test-cream.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed replacement changed the existing product")
	}
}

func TestImportInventoryValidatesEveryProductBeforeWriting(t *testing.T) {
	root := t.TempDir()
	localDir := filepath.Join(root, "local")
	store, err := Open(Options{
		DataDir:  filepath.Join("..", "..", "data"),
		LocalDir: localDir,
		CacheDir: filepath.Join(root, "cache"),
		Offline:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	falseValue := false
	manifest := inventoryManifest{
		SchemaVersion: 1,
		Products: []inventoryProduct{
			{
				ProductKey: "local:first-product", Brand: "Test", Name: "First", UnitsOwned: 1,
				INCI: "Aqua, Glycerin, Niacinamide", PanelType: "water-based",
				Source: &inventorySource{Name: "manufacturer", URL: "https://example.com/p", FetchedAt: stringPointer("2026-10-26T14:00:00Z"), Method: "manufacturer_page", Confidence: 0.8, RobotsAllowed: boolPointer(true)},
			},
			{
				ProductKey: "local:second-product", Brand: "Test", Name: "Second", UnitsOwned: 1,
				INCI: "Aqua, Glycerin, Niacinamide", PanelType: "water-based",
				Source: &inventorySource{Name: "retailer", URL: "https://example.com/p2", FetchedAt: stringPointer("2026-10-26T14:00:00Z"), Method: "retailer_page", Confidence: 0.8, RobotsAllowed: &falseValue},
			},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "inventory.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportInventory(path, "", false, false); err == nil {
		t.Fatal("import accepted a source host denied by robots.txt")
	}
	if _, err := os.Stat(filepath.Join(localDir, "products")); !os.IsNotExist(err) {
		t.Fatalf("invalid batch wrote product files: %v", err)
	}
}

func TestImportInventoryAcceptsWaterBasedFormulaWithoutSeparateWaterToken(t *testing.T) {
	root := t.TempDir()
	store, err := Open(Options{
		DataDir:  filepath.Join("..", "..", "data"),
		LocalDir: filepath.Join(root, "local"),
		CacheDir: filepath.Join(root, "cache"),
		Offline:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "schema_version": 1,
  "products": [{
    "product_key": "local:snail-cream",
    "brand": "Test",
    "name": "Snail cream",
    "units_owned": 1,
    "inci": "Snail Secretion Filtrate, Glycerin, Niacinamide",
    "panel_type": "water-based",
    "source": {
      "source": "manufacturer",
      "url": "https://manufacturer.example/snail-cream",
      "fetched_at": null,
      "method": "manufacturer_page",
      "confidence": 0.8,
      "robots_allowed": true,
      "notes": ["Source checked 2026-10-26; exact retrieval time not retained."]
    }
  }]
}`
	path := filepath.Join(root, "inventory.json")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportInventory(path, "", false, false); err != nil {
		t.Fatalf("source-listed filtrate formula with unknown fetch time was rejected: %v", err)
	}
	product := store.ProductByKey("local:snail-cream")
	if product == nil || product.Source == nil || product.Source.FetchedAt != nil {
		t.Fatalf("unknown fetch time should remain null: %#v", product)
	}
	if data, err := os.ReadFile(filepath.Join(root, "local", "products", "local_snail-cream.json")); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(data), `"fetched_at": null`) {
		t.Fatalf("serialized unknown fetch time is not null: %s", data)
	}
}

func boolPointer(value bool) *bool { return &value }

func stringPointer(value string) *string { return &value }
