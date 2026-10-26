package inci

import "testing"

func fixtureResolver() *Resolver {
	records := []IngredientRecord{
		{INCIName: "AQUA", Functions: []string{"SOLVENT"}},
		{INCIName: "GLYCERIN", Functions: []string{"HUMECTANT", "SOLVENT"}},
		{INCIName: "NIACINAMIDE", Functions: []string{"SMOOTHING"}},
		{INCIName: "PARFUM", Functions: []string{"FRAGRANCE"}},
		{INCIName: "SODIUM BENZOATE", Functions: []string{"PRESERVATIVE"}},
	}
	entries := []AnnexEntry{{
		Annex: "II", Reference: "1388", ChemicalName: "Octamethylcyclotetrasiloxane; D4",
		Names:           []string{"CYCLOMETHICONE"},
		NormalisedNames: []NormalizedName{{Name: "CYCLOMETHICONE", Source: "identified"}},
	}}
	return NewResolver(records, entries)
}

func TestResolverChain(t *testing.T) {
	r := fixtureResolver()
	tests := []struct {
		input     string
		canonical string
		method    string
	}{
		{"Aqua", "AQUA", "exact"},
		{"Water", "AQUA", "synonym_map"},
		{"Glycerine", "GLYCERIN", "synonym_map"},
		{"Niacinamide", "NIACINAMIDE", "exact"},
		{"Cyclomethicone", "CYCLOMETHICONE", "exact"},
		{"Aqua (Water)", "AQUA", "paren_stripped"},
		{"Fragrance", "PARFUM", "synonym_map"},
	}
	for _, test := range tests {
		items := Parse(test.input)
		if len(items) != 1 {
			t.Fatalf("%q parsed to %d items", test.input, len(items))
		}
		got := r.Resolve(items[0])
		if got.Canonical == nil || *got.Canonical != test.canonical {
			t.Fatalf("%q -> %#v, want %s", test.input, got.Canonical, test.canonical)
		}
		if got.Method != test.method {
			t.Fatalf("%q method = %q, want %q", test.input, got.Method, test.method)
		}
	}
}

func TestResolverAnnexOnlyAndMiss(t *testing.T) {
	r := fixtureResolver()
	items := Parse("Cyclomethicone, Unobtainium")
	lines, unresolved := r.ResolveAll(items)
	if len(lines) != 2 || len(unresolved) != 1 {
		t.Fatalf("lines=%d unresolved=%d", len(lines), len(unresolved))
	}
	if lines[0].Annex == nil || lines[0].Annex.Matches[0].Reference != "1388" {
		t.Fatalf("annex-only match = %#v", lines[0])
	}
	if lines[1].Canonical != nil || lines[1].Method != "unresolved" {
		t.Fatalf("miss = %#v", lines[1])
	}
}
