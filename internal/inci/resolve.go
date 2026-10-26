package inci

import (
	"sort"
	"strings"
)

// Resolver is the deterministic exact-name chain. It never edits a name to
// make it match: every step is a recorded transformation, and a miss stays a
// miss.
type Resolver struct {
	byName   map[string][]IngredientRecord
	annex    map[string][]AnnexMatch
	synonyms map[string]string
	typos    map[string]string
}

func NewResolver(records []IngredientRecord, entries []AnnexEntry) *Resolver {
	r := &Resolver{
		byName:   map[string][]IngredientRecord{},
		annex:    map[string][]AnnexMatch{},
		synonyms: map[string]string{},
		typos:    map[string]string{},
	}
	for _, record := range records {
		r.addRecord(record.INCIName, record)
		if record.INNName != nil {
			r.addRecord(*record.INNName, record)
		}
		if record.PhEurName != nil {
			r.addRecord(*record.PhEurName, record)
		}
	}
	for _, entry := range entries {
		for _, name := range entry.NormalisedNames {
			match := AnnexMatch{
				Annex:            entry.Annex,
				Reference:        entry.Reference,
				ChemicalName:     entry.ChemicalName,
				MatchedName:      name.Name,
				NameSource:       name.Source,
				ProductTypes:     entry.ProductTypes,
				MaxConcentration: entry.MaxConcentration,
				Warnings:         entry.Wording,
				Qualifier:        entry.Qualifier,
				NanoOnly:         entry.NanoOnly,
			}
			r.annex[Fold(name.Name)] = appendUniqueMatch(r.annex[Fold(name.Name)], match)
		}
	}

	// Label spellings that are not the INCI name. These are deliberately few
	// and one-way; a broad synonym map is how false annex matches are born.
	r.synonyms = map[string]string{
		"water":              "aqua",
		"fragrance":          "parfum",
		"perfume":            "parfum",
		"glycerine":          "glycerin",
		"denatured alcohol":  "alcohol denat",
		"vitamin c":          "ascorbic acid",
		"vitamin e":          "tocopherol",
		"sodium hyaluronate": "sodium hyaluronate",
	}
	r.typos = map[string]string{
		"glycérine":     "glycerine",
		"glycerin ":     "glycerin",
		"panthenole":    "panthenol",
		"niacinamid":    "niacinamide",
		"salicylicacid": "salicylic acid",
	}
	return r
}

func (r *Resolver) addRecord(name string, record IngredientRecord) {
	key := Fold(name)
	if key == "" {
		return
	}
	for _, existing := range r.byName[key] {
		if existing.INCIName == record.INCIName {
			return
		}
	}
	r.byName[key] = append(r.byName[key], record)
}

func appendUniqueMatch(matches []AnnexMatch, match AnnexMatch) []AnnexMatch {
	for _, existing := range matches {
		if existing.Annex == match.Annex && existing.Reference == match.Reference &&
			existing.MatchedName == match.MatchedName {
			return matches
		}
	}
	return append(matches, match)
}

type lookupResult struct {
	canonical *string
	functions []string
	annex     *AnnexStatus
}

func (r *Resolver) lookup(name string) (lookupResult, bool) {
	key := Fold(name)
	records := r.byName[key]
	matches := r.annex[key]
	if len(records) == 0 && len(matches) == 0 {
		return lookupResult{}, false
	}
	res := lookupResult{}
	if len(records) > 0 {
		canonical := records[0].INCIName
		res.canonical = &canonical
		seen := map[string]bool{}
		for _, record := range records {
			for _, function := range record.Functions {
				if function != "" && !seen[function] {
					seen[function] = true
					res.functions = append(res.functions, function)
				}
			}
		}
		sort.Strings(res.functions)
	}
	if len(matches) > 0 {
		if res.canonical == nil && matches[0].MatchedName != "" {
			canonical := matches[0].MatchedName
			res.canonical = &canonical
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Annex != matches[j].Annex {
				return matches[i].Annex < matches[j].Annex
			}
			return matches[i].Reference < matches[j].Reference
		})
		res.annex = &AnnexStatus{Matches: matches}
	}
	return res, true
}

type candidate struct {
	text       string
	method     string
	confidence float64
}

func (r *Resolver) candidates(ing Ingredient) []candidate {
	var out []candidate
	seen := map[string]bool{}
	add := func(text, method string, confidence float64) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		key := Fold(text)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, candidate{text: text, method: method, confidence: confidence})
	}

	add(ing.Printed, "exact", 1.0)
	add(cleanToken(ing.Printed), "cleaned", 0.98)
	if mapped, ok := r.typos[Fold(ing.Printed)]; ok {
		add(mapped, "typo_map", 0.9)
	}
	add(percentRE.ReplaceAllString(ing.Printed, " "), "percent_stripped", 0.95)

	// Parenthetical asides: the whole name, then the contents.
	if stripped := parentheticalRE.ReplaceAllString(ing.Printed, " "); strings.TrimSpace(stripped) != "" {
		add(stripped, "paren_stripped", 0.85)
	}
	for _, match := range parentheticalRE.FindAllStringSubmatch(ing.Printed, -1) {
		for _, piece := range strings.Split(match[1], "/") {
			add(piece, "paren_stripped", 0.82)
		}
	}
	for _, part := range synonymSlashRE.Split(ing.Printed, -1) {
		if strings.TrimSpace(part) != "" && Fold(part) != Fold(ing.Printed) {
			add(part, "slash_variant", 0.8)
		}
	}
	for _, alias := range ing.Aliases {
		switch alias.Kind {
		case "whole":
			add(alias.Text, "paren_stripped", 0.85)
		case "part":
			add(alias.Text, "slash_variant", 0.78)
		}
	}
	for _, item := range append([]candidate(nil), out...) {
		if mapped, ok := r.synonyms[Fold(item.text)]; ok {
			add(mapped, "synonym_map", 0.9)
		}
	}
	return out
}

// Resolve runs the chain and always returns a resolution, even a miss.
func (r *Resolver) Resolve(ing Ingredient) Resolution {
	for _, cand := range r.candidates(ing) {
		if result, ok := r.lookup(cand.text); ok {
			functions := result.functions
			if functions == nil {
				functions = []string{}
			}
			return Resolution{
				Printed:    ing.Printed,
				Canonical:  result.canonical,
				Functions:  functions,
				Annex:      result.annex,
				Method:     cand.method,
				Confidence: cand.confidence,
			}
		}
	}
	return Resolution{
		Printed:    ing.Printed,
		Functions:  []string{},
		Method:     "unresolved",
		Confidence: 0,
	}
}

func (r *Resolver) ResolveAll(ingredients []Ingredient) ([]Line, []Unresolved) {
	lines := make([]Line, 0, len(ingredients))
	var unresolved []Unresolved
	for _, ing := range ingredients {
		res := r.Resolve(ing)
		lines = append(lines, Line{
			Position:    ing.Index,
			Printed:     ing.Printed,
			Canonical:   res.Canonical,
			Functions:   res.Functions,
			Annex:       res.Annex,
			DeclaredPct: ing.DeclaredPct,
			Method:      res.Method,
			Confidence:  res.Confidence,
		})
		if res.Canonical == nil {
			unresolved = append(unresolved, Unresolved{
				Printed:  ing.Printed,
				Position: ing.Index,
				Section:  ing.Position,
				Reason:   "not in the CosIng glossary or annexes",
			})
		}
	}
	return lines, unresolved
}

func (r *Resolver) IngredientRecord(name string) (*IngredientRecord, []AnnexMatch) {
	key := Fold(name)
	records := r.byName[key]
	if len(records) == 0 && len(r.annex[key]) == 0 {
		return nil, nil
	}
	if len(records) == 0 {
		return nil, r.annex[key]
	}
	record := records[0]
	return &record, r.annex[key]
}
