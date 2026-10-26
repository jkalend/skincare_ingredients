package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/jkalend/skincare_ingredients/internal/inci"
	"github.com/jkalend/skincare_ingredients/internal/store"
)

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WorkOrder struct {
	ID          string              `json:"id"`
	Kind        string              `json:"kind"`
	Reason      string              `json:"reason"`
	Blocking    bool                `json:"blocking"`
	Payload     map[string]any      `json:"payload"`
	Suggested   []map[string]string `json:"suggested"`
	Constraints map[string]any      `json:"constraints"`
}

type Response struct {
	Data       any
	Exit       int
	Warnings   []Warning
	WorkOrders []WorkOrder
	Next       []string
}

type Analysis struct {
	Ingredients []inci.Line       `json:"ingredients"`
	MayContain  []inci.Line       `json:"may_contain"`
	Unresolved  []inci.Unresolved `json:"unresolved"`
}

type Service struct {
	Store *store.Store
}

func New(s *store.Store) *Service {
	return &Service{Store: s}
}

func (s *Service) Registry() inci.RegistryInfo {
	return s.Store.Registry.Info()
}

func (s *Service) warnings(list ...Warning) []Warning {
	if list == nil {
		return []Warning{}
	}
	return list
}

func workOrderID(kind, key string) string {
	sum := sha1.Sum([]byte(kind + "|" + key))
	return "wo_" + hex.EncodeToString(sum[:])[:12]
}

// expandDottedTokens handles the common OBF label shape "xanthan gum. allantoin.
// dipotassium glycyrrhizate": the panel uses periods where the parser expects
// commas. Only expand when every part resolves, so prose sentences are not
// shredded into fake ingredients.
func (s *Service) expandDottedTokens(parsed []inci.Ingredient) []inci.Ingredient {
	resolver := s.Store.Registry.Resolver
	var out []inci.Ingredient
	for _, ing := range parsed {
		if strings.Contains(ing.Printed, ". ") && resolver.Resolve(ing).Canonical == nil {
			parts := strings.Split(ing.Printed, ". ")
			expanded := make([]inci.Ingredient, 0, len(parts))
			all := true
			for _, part := range parts {
				sub := inci.Parse(part)
				if len(sub) != 1 || resolver.Resolve(sub[0]).Canonical == nil {
					all = false
					break
				}
				expanded = append(expanded, sub[0])
			}
			if all && len(expanded) > 1 {
				out = append(out, expanded...)
				continue
			}
		}
		out = append(out, ing)
	}
	reindex := func(position inci.Position) {
		index := 0
		for i := range out {
			if out[i].Position == position {
				index++
				out[i].Index = index
			}
		}
	}
	reindex(inci.Declared)
	reindex(inci.MayContain)
	return out
}

func (s *Service) AnalyzeText(text string) (*Analysis, []Warning, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil, fmt.Errorf("empty ingredient text")
	}
	block, _ := inci.Extract(text)
	if block == "" {
		block, _ = inci.Trim(text)
	}
	if strings.TrimSpace(block) == "" {
		return nil, nil, fmt.Errorf("no ingredient list found")
	}
	parsed := s.expandDottedTokens(inci.Parse(block))
	if len(parsed) == 0 {
		return nil, nil, fmt.Errorf("no ingredient list found")
	}
	var declared, may []inci.Ingredient
	for _, ing := range parsed {
		if ing.Position == inci.MayContain {
			may = append(may, ing)
		} else {
			declared = append(declared, ing)
		}
	}
	declaredLines, declaredUnresolved := s.Store.Registry.Resolver.ResolveAll(declared)
	mayLines, mayUnresolved := s.Store.Registry.Resolver.ResolveAll(may)
	unresolved := append(declaredUnresolved, mayUnresolved...)
	if unresolved == nil {
		unresolved = []inci.Unresolved{}
	}
	if declaredLines == nil {
		declaredLines = []inci.Line{}
	}
	if mayLines == nil {
		mayLines = []inci.Line{}
	}
	warnings := []Warning{}
	prose := false
	for _, ing := range parsed {
		if inci.LooksLikeProse(ing) {
			prose = true
			break
		}
	}
	if prose {
		warnings = append(warnings, Warning{
			Code:    "prose_detected",
			Message: "at least one parsed item looks like pack prose, not an INCI name",
		})
	}
	if len(unresolved) > 0 {
		warnings = append(warnings, Warning{
			Code:    "unresolved_tokens",
			Message: fmt.Sprintf("%d token(s) were kept verbatim and not resolved", len(unresolved)),
		})
	}
	return &Analysis{
		Ingredients: declaredLines,
		MayContain:  mayLines,
		Unresolved:  unresolved,
	}, warnings, nil
}

func (s *Service) Analyze(text string) *Response {
	analysis, warnings, err := s.AnalyzeText(text)
	if err != nil {
		return &Response{Data: map[string]any{"error": err.Error()}, Exit: 2, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{"parse --text \"Aqua, Glycerin\""}}
	}
	exit := 0
	if len(analysis.Unresolved) > 0 {
		exit = 1
	}
	return &Response{Data: analysis, Exit: exit, Warnings: s.warnings(warnings...), WorkOrders: []WorkOrder{}, Next: []string{}}
}

var barcodeRE = regexp.MustCompile(`^\d{8,14}$`)

func (s *Service) buildProduct(obf *store.OBFProduct) (*inci.Product, *Analysis, []Warning, error) {
	block, _ := inci.Trim(obf.Ingredients)
	parsed := s.expandDottedTokens(inci.Parse(block))
	if len(parsed) == 0 {
		return nil, nil, nil, store.ErrNoPanel
	}
	var declared, may []inci.Ingredient
	for _, ing := range parsed {
		if ing.Position == inci.MayContain {
			may = append(may, ing)
		} else {
			declared = append(declared, ing)
		}
	}
	declaredLines, declaredUnresolved := s.Store.Registry.Resolver.ResolveAll(declared)
	mayLines, mayUnresolved := s.Store.Registry.Resolver.ResolveAll(may)
	if declaredLines == nil {
		declaredLines = []inci.Line{}
	}
	if mayLines == nil {
		mayLines = []inci.Line{}
	}
	unresolved := append(declaredUnresolved, mayUnresolved...)
	if unresolved == nil {
		unresolved = []inci.Unresolved{}
	}
	product := &inci.Product{
		ProductKey:  "ean:" + obf.Barcode,
		Barcode:     obf.Barcode,
		Brand:       obf.Brand,
		Name:        obf.Name,
		Size:        obf.Size,
		Ingredients: declaredLines,
		MayContain:  mayLines,
		Unresolved:  unresolved,
		Trust:       "unverified",
		Sources:     []string{"obf:" + obf.Barcode},
		Source: &inci.SourceRef{
			Source:     "obf",
			URL:        obf.URL,
			FetchedAt:  obf.FetchedAt,
			Method:     "obf_api",
			Confidence: 0.7,
		},
		Generated:        true,
		GeneratorVersion: store.GeneratorVersion,
	}
	analysis := &Analysis{Ingredients: declaredLines, MayContain: mayLines, Unresolved: unresolved}
	warnings := []Warning{{
		Code:    "single_source",
		Message: "one Open Beauty Facts observation; trust is unverified",
	}}
	if len(unresolved) > 0 {
		warnings = append(warnings, Warning{
			Code:    "unresolved_tokens",
			Message: fmt.Sprintf("%d token(s) were kept verbatim and not resolved", len(unresolved)),
		})
	}
	return product, analysis, warnings, nil
}

func (s *Service) Lookup(ctx context.Context, barcode string, refresh bool) *Response {
	barcode = strings.TrimSpace(barcode)
	if !barcodeRE.MatchString(barcode) {
		return &Response{Data: map[string]any{"error": "barcode must be 8-14 digits"}, Exit: 2, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
	}
	if !refresh {
		if product := s.Store.ProductByBarcode(barcode); product != nil {
			exit := 0
			if product.Trust != "verified" && product.Trust != "manual" {
				exit = 1
			}
			return &Response{Data: product, Exit: exit, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
		}
	}
	if s.Store.Offline {
		return &Response{Data: map[string]any{"status": "not_found", "barcode": barcode}, Exit: 3, Warnings: []Warning{{Code: "offline", Message: "not cached locally; --offline forbids the OBF lookup"}}, WorkOrders: []WorkOrder{}, Next: []string{"lookup " + barcode + " (online)"}}
	}
	obf, err := s.Store.FetchOBF(ctx, barcode)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			return &Response{Data: map[string]any{"status": "not_found", "barcode": barcode}, Exit: 3, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{"parse --file label.txt"}}
		case store.ErrNoPanel:
			wo := WorkOrder{
				ID:          workOrderID("manual_label", barcode),
				Kind:        "manual_label",
				Reason:      "no_panel",
				Blocking:    true,
				Payload:     map[string]any{"product_key": "ean:" + barcode, "barcode": barcode, "tried": []string{"openbeautyfacts"}},
				Suggested:   []map[string]string{{"action": "photo", "why": "supply a label photo or a pasted INCI list"}},
				Constraints: map[string]any{"max_requests": 3, "must_record_provenance": true},
			}
			return &Response{Data: map[string]any{"status": "no_panel", "barcode": barcode}, Exit: 4, Warnings: []Warning{{Code: "no_panel", Message: "OBF has the product but not its ingredient list"}}, WorkOrders: []WorkOrder{wo}, Next: []string{"parse --file label.txt"}}
		default:
			return &Response{Data: map[string]any{"error": err.Error()}, Exit: 10, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
		}
	}
	product, _, warnings, err := s.buildProduct(obf)
	if err != nil {
		return &Response{Data: map[string]any{"error": err.Error()}, Exit: 10, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
	}
	if err := s.Store.SaveProduct(product); err != nil {
		warnings = append(warnings, Warning{
			Code:    "cache_write_failed",
			Message: "product was returned but could not be cached: " + err.Error(),
		})
	}
	return &Response{Data: product, Exit: 1, Warnings: s.warnings(warnings...), WorkOrders: []WorkOrder{}, Next: []string{"analyze --product " + product.ProductKey}}
}

func (s *Service) Search(query string, limit int) *Response {
	results := s.Store.Search(query, limit)
	return &Response{Data: map[string]any{"query": query, "results": results}, Exit: 0, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}

func (s *Service) Ingredient(name string) *Response {
	record, matches := s.Store.Registry.Resolver.IngredientRecord(name)
	if record == nil && len(matches) == 0 {
		return &Response{Data: map[string]any{"status": "not_found", "query": name}, Exit: 3, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{"resolve " + name}}
	}
	status := ""
	var ingredient any
	if record == nil {
		status = "annex_only"
	} else {
		ingredient = record
	}
	return &Response{
		Data: map[string]any{
			"status":     status,
			"ingredient": ingredient,
			"annex":      matches,
		},
		Exit:       0,
		Warnings:   []Warning{},
		WorkOrders: []WorkOrder{},
		Next:       []string{},
	}
}

func (s *Service) Resolve(tokens []string) *Response {
	resolved := make([]inci.Resolution, 0, len(tokens))
	for _, token := range tokens {
		parsed := inci.Parse(token)
		if len(parsed) == 0 {
			resolved = append(resolved, inci.Resolution{
				Printed: token, Functions: []string{}, Method: "unresolved", Confidence: 0,
			})
			continue
		}
		resolved = append(resolved, s.Store.Registry.Resolver.Resolve(parsed[0]))
	}
	exit := 0
	for _, item := range resolved {
		if item.Canonical == nil {
			exit = 1
		}
	}
	return &Response{Data: map[string]any{"tokens": resolved}, Exit: exit, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}

func (s *Service) ShelfAdd(name, barcode string) *Response {
	lookup := s.Lookup(context.Background(), barcode, false)
	if lookup.Exit != 0 && lookup.Exit != 1 {
		return lookup
	}
	product, ok := lookup.Data.(*inci.Product)
	if !ok {
		return lookup
	}
	shelf, err := s.Store.AddToShelf(name, product.ProductKey)
	if err != nil {
		return &Response{Data: map[string]any{"error": err.Error()}, Exit: 10, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
	}
	return &Response{Data: shelf, Exit: lookup.Exit, Warnings: lookup.Warnings, WorkOrders: lookup.WorkOrders, Next: []string{"shelf ls --shelf " + shelf.Name}}
}

func (s *Service) ShelfList(name string) *Response {
	shelf := s.Store.Shelf(name)
	products := s.Store.ShelfProducts(name)
	items := make([]map[string]any, 0, len(products))
	for _, item := range products {
		items = append(items, map[string]any{
			"product_key": item.Product.ProductKey,
			"barcode":     item.Product.Barcode,
			"brand":       item.Product.Brand,
			"name":        item.Product.Name,
			"trust":       item.Product.Trust,
		})
	}
	return &Response{Data: map[string]any{"shelf": shelf, "items": items}, Exit: 0, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}

func (s *Service) ShelfRemove(name, barcode string) *Response {
	product := s.Store.ProductByBarcode(barcode)
	if product == nil {
		return &Response{Data: map[string]any{"status": "not_found", "barcode": barcode}, Exit: 3, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
	}
	shelf, err := s.Store.RemoveFromShelf(name, product.ProductKey)
	if err != nil {
		return &Response{Data: map[string]any{"error": err.Error()}, Exit: 10, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
	}
	return &Response{Data: shelf, Exit: 0, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}

func (s *Service) ShelfReport(name string) *Response {
	return &Response{Data: s.Store.ShelfReport(name), Exit: 0, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}

func (s *Service) Export(shelfName, format, outPath string) *Response {
	result, err := s.Store.Export(shelfName, format, outPath)
	if err != nil {
		return &Response{Data: map[string]any{"error": err.Error()}, Exit: 2, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
	}
	return &Response{Data: result, Exit: 0, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}

func (s *Service) Doctor() *Response {
	return &Response{Data: s.Store.Doctor(), Exit: 0, Warnings: []Warning{}, WorkOrders: []WorkOrder{}, Next: []string{}}
}
