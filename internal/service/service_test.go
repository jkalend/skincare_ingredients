package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jkalend/skincare_ingredients/internal/inci"
	"github.com/jkalend/skincare_ingredients/internal/testutil"
)

func TestAnalyzeResolvesAndCites(t *testing.T) {
	svc := New(testutil.Store(t))
	resp := svc.Analyze("Aqua, Glycerin, Cyclomethicone, Unobtainium")
	if resp.Exit != 1 {
		t.Fatalf("exit = %d, want 1 (one unresolved token)", resp.Exit)
	}
	analysis, ok := resp.Data.(*Analysis)
	if !ok {
		t.Fatalf("data type = %T", resp.Data)
	}
	if len(analysis.Ingredients) != 4 {
		t.Fatalf("got %d lines", len(analysis.Ingredients))
	}
	if analysis.Ingredients[0].Canonical == nil || *analysis.Ingredients[0].Canonical != "AQUA" {
		t.Fatalf("Aqua did not resolve: %#v", analysis.Ingredients[0])
	}
	cyclo := analysis.Ingredients[2]
	if cyclo.Annex == nil || len(cyclo.Annex.Matches) != 1 || cyclo.Annex.Matches[0].Reference != "1388" {
		t.Fatalf("annex citation missing: %#v", cyclo)
	}
	if len(analysis.Unresolved) != 1 || analysis.Unresolved[0].Printed != "Unobtainium" {
		t.Fatalf("unresolved = %#v", analysis.Unresolved)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected an unresolved warning")
	}
}

func TestAnalyzeSynonymAndMayContain(t *testing.T) {
	svc := New(testutil.Store(t))
	resp := svc.Analyze("Water, Glycerine, Fragrance. May contain: Limonene")
	analysis := resp.Data.(*Analysis)
	if analysis.Ingredients[0].Canonical == nil || *analysis.Ingredients[0].Canonical != "AQUA" {
		t.Fatalf("Water did not map to AQUA: %#v", analysis.Ingredients[0])
	}
	if analysis.Ingredients[1].Canonical == nil || *analysis.Ingredients[1].Canonical != "GLYCERIN" {
		t.Fatalf("Glycerine did not map to GLYCERIN: %#v", analysis.Ingredients[1])
	}
	if len(analysis.MayContain) != 1 || analysis.MayContain[0].Printed != "Limonene" {
		t.Fatalf("may contain = %#v", analysis.MayContain)
	}
}

func TestLookupOffline(t *testing.T) {
	svc := New(testutil.Store(t))
	resp := svc.Lookup(context.Background(), "1000034026006", false)
	if resp.Exit != 3 {
		t.Fatalf("exit = %d, want 3", resp.Exit)
	}
	if len(resp.Warnings) == 0 || resp.Warnings[0].Code != "offline" {
		t.Fatalf("warnings = %#v", resp.Warnings)
	}
}

func TestShelfLifecycle(t *testing.T) {
	svc := New(testutil.Store(t))
	product := &inci.Product{
		ProductKey: "ean:1000034026006",
		Barcode:    "1000034026006",
		Brand:      "Test",
		Name:       "Cream",
		Trust:      "unverified",
		Source:     &inci.SourceRef{Source: "obf"},
		Ingredients: []inci.Line{{
			Position: 1, Printed: "Aqua", Canonical: new("AQUA"), Functions: []string{"SOLVENT"}, Confidence: 1,
		}},
		MayContain: []inci.Line{}, Unresolved: []inci.Unresolved{}, Sources: []string{"obf:test"},
	}
	if err := svc.Store.SaveProduct(product); err != nil {
		t.Fatal(err)
	}
	add := svc.ShelfAdd("default", "1000034026006")
	if add.Exit != 1 {
		t.Fatalf("shelf add exit = %d", add.Exit)
	}
	list := svc.ShelfList("default")
	data := list.Data.(map[string]any)
	if len(data["items"].([]map[string]any)) != 1 {
		t.Fatalf("shelf list = %#v", data)
	}
	report := svc.ShelfReport("default")
	if report.Exit != 0 {
		t.Fatalf("report exit = %d", report.Exit)
	}
	remove := svc.ShelfRemove("default", "1000034026006")
	if remove.Exit != 0 {
		t.Fatalf("remove exit = %d", remove.Exit)
	}

	localProduct := &inci.Product{
		ProductKey: "local:test-cream", Brand: "Test", Name: "Private cream", UnitsOwned: 1,
		Trust: "unverified", Ingredients: []inci.Line{}, MayContain: []inci.Line{},
		Unresolved: []inci.Unresolved{}, Sources: []string{},
	}
	if err := svc.Store.SaveProduct(localProduct); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.AddToShelf("default", localProduct.ProductKey); err != nil {
		t.Fatal(err)
	}
	remove = svc.ShelfRemove("default", localProduct.ProductKey)
	if remove.Exit != 0 {
		t.Fatalf("remove by product key exit = %d", remove.Exit)
	}
}

func TestExport(t *testing.T) {
	svc := New(testutil.Store(t))
	product := &inci.Product{
		ProductKey: "ean:1000034026006", Barcode: "1000034026006", Name: "Cream", Trust: "unverified",
		Source:      &inci.SourceRef{Source: "obf"},
		Ingredients: []inci.Line{{Position: 1, Printed: "Aqua", Canonical: new("AQUA"), Functions: []string{"SOLVENT"}, Confidence: 1}},
		MayContain:  []inci.Line{}, Unresolved: []inci.Unresolved{}, Sources: []string{"obf:test"},
	}
	if err := svc.Store.SaveProduct(product); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.AddToShelf("default", product.ProductKey); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, format := range []string{"json", "csv"} {
		out := filepath.Join(dir, "export."+format)
		resp := svc.Export("default", format, out)
		if resp.Exit != 0 {
			t.Fatalf("%s export exit = %d, data=%#v", format, resp.Exit, resp.Data)
		}
		if _, err := os.Stat(out); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIngredientAnnexOnly(t *testing.T) {
	svc := New(testutil.Store(t))
	resp := svc.Ingredient("Cyclomethicone")
	if resp.Exit != 0 {
		t.Fatalf("exit = %d", resp.Exit)
	}
	data := resp.Data.(map[string]any)
	if data["ingredient"] != nil {
		t.Fatalf("expected no ingredient record: %#v", data)
	}
	matches := data["annex"].([]inci.AnnexMatch)
	if len(matches) != 1 || matches[0].Reference != "1388" {
		t.Fatalf("annex = %#v", matches)
	}
}
