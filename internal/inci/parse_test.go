package inci

import (
	"testing"
)

func TestFold(t *testing.T) {
	cases := map[string]string{
		"  Sodium   Benzoate ": "sodium benzoate",
		"C.I. 77491":           "ci 77491",
		"Cl 77492":             "ci 77492",
		"Aluminium Sulphate":   "aluminum sulfate",
		"ANTHRA- 9,10-QUINONE": "anthra-9,10-quinone",
		"L’Oréal":              "l'oréal",
		"AQUA":                 "aqua",
	}
	for input, want := range cases {
		if got := Fold(input); got != want {
			t.Errorf("Fold(%q) = %q, want %q", input, got, want)
		}
	}
	if Fold(Fold("Niacinamide")) != Fold("Niacinamide") {
		t.Fatal("Fold is not idempotent")
	}
}

func TestParsePlainList(t *testing.T) {
	items := Parse("Aqua, Glycerin, Niacinamide")
	if len(items) != 3 {
		t.Fatalf("got %d tokens, want 3: %#v", len(items), items)
	}
	if items[0].Printed != "Aqua" || items[2].Printed != "Niacinamide" {
		t.Fatalf("unexpected tokens: %#v", items)
	}
	for _, item := range items {
		if item.Position != Declared || item.Index < 1 {
			t.Fatalf("bad position: %#v", item)
		}
	}
}

func TestParseBracketedComma(t *testing.T) {
	items := Parse("Aqua, Glycerin, Butyrospermum Parkii (Shea, Karite) Butter")
	if len(items) != 3 {
		t.Fatalf("got %d tokens, want 3: %#v", len(items), items)
	}
	if items[2].Printed != "Butyrospermum Parkii (Shea, Karite) Butter" {
		t.Fatalf("comma inside parentheses was split: %q", items[2].Printed)
	}
}

func TestParseTerminatesAtDirections(t *testing.T) {
	items := Parse("Ingredients: Aqua, Glycerin, Sodium Chloride. Directions: apply to skin")
	if len(items) != 3 {
		t.Fatalf("got %d tokens, want 3: %#v", len(items), items)
	}
	for _, item := range items {
		if item.Printed == "Directions" || item.Printed == "apply to skin" {
			t.Fatalf("panel prose leaked into ingredients: %#v", items)
		}
	}
}

func TestParseMayContain(t *testing.T) {
	items := Parse("Aqua, Glycerin, [+/- CI 77491, CI 77492] Talc")
	declared, may := 0, 0
	for _, item := range items {
		if item.Position == MayContain {
			may++
		} else {
			declared++
		}
	}
	if declared != 3 || may != 2 {
		t.Fatalf("declared=%d may=%d, want 3/2: %#v", declared, may, items)
	}
}

func TestParsePercent(t *testing.T) {
	items := Parse("Aqua, Sodium Benzoate 0.5%, Glycerin")
	if len(items) != 3 {
		t.Fatalf("got %d tokens", len(items))
	}
	if items[1].DeclaredPct == nil || *items[1].DeclaredPct != 0.5 {
		t.Fatalf("declared percentage not captured: %#v", items[1])
	}
	if items[1].Printed != "Sodium Benzoate" {
		t.Fatalf("percentage was not removed from the name: %q", items[1].Printed)
	}
}

func TestParseBlendAndHTML(t *testing.T) {
	items := Parse("Aqua, Glycerin (and) Panthenol")
	if len(items) != 3 {
		t.Fatalf("got %d tokens, want 3: %#v", len(items), items)
	}
}

func TestExtractWholeLabel(t *testing.T) {
	block, ok := Extract("Luxury cream.\nIngredients: Aqua, Glycerin. Use daily.")
	if !ok {
		t.Fatal("Extract did not find a panel")
	}
	items := Parse(block)
	if len(items) != 2 {
		t.Fatalf("got %d tokens, want 2: %#v", len(items), items)
	}
}
