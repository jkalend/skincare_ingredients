package store

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jkalend/skincare_ingredients/internal/inci"
)

const GeneratorVersion = "0.1.0"

type Options struct {
	DataDir    string
	LocalDir   string
	CacheDir   string
	Offline    bool
	PublicOnly bool
	Timeout    time.Duration
	UserAgent  string
}

type Store struct {
	Options
	Registry  *Registry
	Products  map[string]*inci.Product
	ByBarcode map[string]string
	Shelves   map[string]*Shelf
}

type Shelf struct {
	Name             string   `json:"name"`
	Items            []string `json:"items"`
	Generated        bool     `json:"generated"`
	GeneratorVersion string   `json:"generator_version"`
}

type SearchResult struct {
	ProductKey  string  `json:"product_key"`
	Barcode     string  `json:"barcode,omitempty"`
	Brand       string  `json:"brand,omitempty"`
	Name        string  `json:"name,omitempty"`
	Score       float64 `json:"score"`
	MatchReason string  `json:"match_reason"`
}

func Open(opts Options) (*Store, error) {
	if opts.DataDir == "" {
		opts.DataDir = "data"
	}
	if opts.LocalDir == "" {
		opts.LocalDir = "local"
	}
	if opts.CacheDir == "" {
		opts.CacheDir = "cache"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 20 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "SkincareIngredients/0.1 (+https://github.com/jkalend/skincare_ingredients; personal research)"
	}
	registry, err := LoadRegistry(filepath.Join(opts.DataDir, "registry", "cosing"))
	if err != nil {
		return nil, err
	}
	s := &Store{
		Options:   opts,
		Registry:  registry,
		Products:  map[string]*inci.Product{},
		ByBarcode: map[string]string{},
		Shelves:   map[string]*Shelf{},
	}
	if err := s.loadProducts(filepath.Join(opts.DataDir, "products")); err != nil {
		return nil, err
	}
	if !opts.PublicOnly {
		if err := s.loadProducts(filepath.Join(opts.LocalDir, "products")); err != nil {
			return nil, err
		}
	}
	if err := s.loadShelves(filepath.Join(opts.DataDir, "shelf")); err != nil {
		return nil, err
	}
	if !opts.PublicOnly {
		if err := s.loadShelves(filepath.Join(opts.LocalDir, "shelf")); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) loadProducts(dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var product inci.Product
		if err := json.Unmarshal(data, &product); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if product.ProductKey == "" {
			continue
		}
		if product.UnitsOwned < 1 {
			product.UnitsOwned = 1
		}
		s.Products[product.ProductKey] = &product
		if product.Barcode != "" {
			s.ByBarcode[product.Barcode] = product.ProductKey
		}
	}
	return nil
}

func (s *Store) loadShelves(dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var shelf Shelf
		if err := json.Unmarshal(data, &shelf); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if shelf.Name == "" {
			continue
		}
		s.Shelves[shelf.Name] = &shelf
	}
	return nil
}

func fileKey(key string) string {
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_")
	return replacer.Replace(key) + ".json"
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) SaveProduct(product *inci.Product) error {
	product.Generated = true
	product.GeneratorVersion = GeneratorVersion
	if product.UnitsOwned < 1 {
		product.UnitsOwned = 1
	}
	dir := filepath.Join(s.LocalDir, "products")
	if product.Source != nil && product.Source.Source == "obf" {
		dir = filepath.Join(s.DataDir, "products")
	}
	if err := writeJSON(filepath.Join(dir, fileKey(product.ProductKey)), product); err != nil {
		return err
	}
	s.Products[product.ProductKey] = product
	if product.Barcode != "" {
		s.ByBarcode[product.Barcode] = product.ProductKey
	}
	return nil
}

func (s *Store) ProductByBarcode(barcode string) *inci.Product {
	key, ok := s.ByBarcode[barcode]
	if !ok {
		return nil
	}
	return s.Products[key]
}

func (s *Store) ProductByKey(key string) *inci.Product {
	return s.Products[key]
}

func (s *Store) Search(query string, limit int) []SearchResult {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return []SearchResult{}
	}
	var out []SearchResult
	for _, product := range s.Products {
		name := strings.ToLower(product.Name)
		brand := strings.ToLower(product.Brand)
		score := 0.0
		reason := ""
		switch {
		case name == needle || brand == needle:
			score, reason = 1.0, "exact"
		case strings.HasPrefix(name, needle) || strings.HasPrefix(brand, needle):
			score, reason = 0.9, "prefix"
		case strings.Contains(name, needle) || strings.Contains(brand, needle):
			score, reason = 0.7, "substring"
		case strings.Contains(strings.ToLower(product.Barcode), needle):
			score, reason = 0.8, "barcode"
		default:
			continue
		}
		out = append(out, SearchResult{
			ProductKey:  product.ProductKey,
			Barcode:     product.Barcode,
			Brand:       product.Brand,
			Name:        product.Name,
			Score:       score,
			MatchReason: reason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) Shelf(name string) *Shelf {
	if name == "" {
		name = "default"
	}
	if shelf, ok := s.Shelves[name]; ok {
		return shelf
	}
	return &Shelf{Name: name, Items: []string{}, Generated: true, GeneratorVersion: GeneratorVersion}
}

func (s *Store) SaveShelf(shelf *Shelf) error {
	if shelf.Name == "" {
		shelf.Name = "default"
	}
	shelf.Generated = true
	shelf.GeneratorVersion = GeneratorVersion
	sort.Strings(shelf.Items)
	shelf.Items = unique(shelf.Items)
	if err := writeJSON(filepath.Join(s.LocalDir, "shelf", fileKey(shelf.Name)), shelf); err != nil {
		return err
	}
	s.Shelves[shelf.Name] = shelf
	return nil
}

func unique(items []string) []string {
	out := items[:0]
	seen := map[string]bool{}
	for _, item := range items {
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) AddToShelf(name, productKey string) (*Shelf, error) {
	shelf := s.Shelf(name)
	shelf.Items = append(shelf.Items, productKey)
	if err := s.SaveShelf(shelf); err != nil {
		return nil, err
	}
	return shelf, nil
}

func (s *Store) RemoveFromShelf(name, productKey string) (*Shelf, error) {
	shelf := s.Shelf(name)
	var kept []string
	for _, item := range shelf.Items {
		if item != productKey {
			kept = append(kept, item)
		}
	}
	shelf.Items = kept
	if err := s.SaveShelf(shelf); err != nil {
		return nil, err
	}
	return shelf, nil
}

type ShelfProduct struct {
	Shelf   string        `json:"shelf"`
	Product *inci.Product `json:"product"`
}

func (s *Store) ShelfProducts(name string) []ShelfProduct {
	shelf := s.Shelf(name)
	var out []ShelfProduct
	for _, key := range shelf.Items {
		product := s.Products[key]
		if product == nil {
			continue
		}
		out = append(out, ShelfProduct{Shelf: shelf.Name, Product: product})
	}
	return out
}

func (s *Store) ShelfReport(name string) map[string]any {
	products := s.ShelfProducts(name)
	canonical := map[string][]string{}
	functions := map[string]int{}
	annex := map[string]int{}
	unresolved := 0
	units := 0
	for _, item := range products {
		units += item.Product.UnitsOwned
		for _, line := range append(append([]inci.Line{}, item.Product.Ingredients...), item.Product.MayContain...) {
			if line.Canonical == nil {
				unresolved++
				continue
			}
			canonical[*line.Canonical] = append(canonical[*line.Canonical], item.Product.ProductKey)
			for _, function := range line.Functions {
				functions[function]++
			}
			if line.Annex != nil {
				for _, match := range line.Annex.Matches {
					annex[match.Annex+" "+match.Reference]++
				}
			}
		}
	}
	var duplicates []map[string]any
	for name, keys := range canonical {
		if len(keys) > 1 {
			duplicates = append(duplicates, map[string]any{"canonical": name, "products": unique(append([]string{}, keys...))})
		}
	}
	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i]["canonical"].(string) < duplicates[j]["canonical"].(string)
	})
	return map[string]any{
		"shelf":       name,
		"products":    len(products),
		"units":       units,
		"ingredients": len(canonical),
		"unresolved":  unresolved,
		"duplicates":  duplicates,
		"functions":   functions,
		"annex":       annex,
	}
}

type ExportResult struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Count  int    `json:"count"`
	Format string `json:"format"`
}

func (s *Store) Export(shelfName, format, outPath string) (*ExportResult, error) {
	if format == "" {
		format = "json"
	}
	if outPath == "" {
		outPath = filepath.Join(s.LocalDir, "exports", "skincare-"+shelfName+"."+format)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return nil, err
	}
	products := s.ShelfProducts(shelfName)
	switch format {
	case "json":
		payload := map[string]any{
			"exported_at": time.Now().UTC().Format(time.RFC3339),
			"shelf":       shelfName,
			"products":    products,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		return &ExportResult{Path: outPath, SHA256: hex.EncodeToString(sum[:]), Count: len(products), Format: format}, nil
	case "csv":
		file, err := os.Create(outPath)
		if err != nil {
			return nil, err
		}
		writer := csv.NewWriter(file)
		_ = writer.Write([]string{"product_key", "units_owned", "brand", "name", "size", "trust", "variant_status", "notes", "source", "source_url", "source_fetched_at", "source_method", "source_confidence", "section", "position", "printed", "canonical", "functions", "annex", "method", "confidence"})
		for _, item := range products {
			product := item.Product
			source, sourceURL, fetchedAt, sourceMethod, sourceConfidence := "", "", "", "", ""
			if product.Source != nil {
				source, sourceURL, sourceMethod = product.Source.Source, product.Source.URL, product.Source.Method
				if product.Source.FetchedAt != nil {
					fetchedAt = *product.Source.FetchedAt
				}
				sourceConfidence = fmt.Sprintf("%.2f", product.Source.Confidence)
			}
			base := []string{product.ProductKey, fmt.Sprint(product.UnitsOwned), product.Brand, product.Name, product.Size, product.Trust, product.VariantStatus, strings.Join(product.Notes, ";"), source, sourceURL, fetchedAt, sourceMethod, sourceConfidence}
			allLines := append(append([]inci.Line{}, product.Ingredients...), product.MayContain...)
			if len(allLines) == 0 {
				_ = writer.Write(append(append([]string{}, base...), []string{"unresolved", "", "", "", "", "", "", "", ""}...))
			}
			for _, line := range allLines {
				canonical := ""
				if line.Canonical != nil {
					canonical = *line.Canonical
				}
				annex := ""
				if line.Annex != nil {
					var refs []string
					for _, match := range line.Annex.Matches {
						refs = append(refs, match.Annex+" "+match.Reference)
					}
					annex = strings.Join(refs, ";")
				}
				section := "declared"
				for _, mayLine := range product.MayContain {
					if mayLine.Position == line.Position && mayLine.Printed == line.Printed {
						section = "may-contain"
						break
					}
				}
				row := append(append([]string{}, base...), section, fmt.Sprint(line.Position), line.Printed, canonical, strings.Join(line.Functions, ";"), annex, line.Method, fmt.Sprintf("%.2f", line.Confidence))
				_ = writer.Write(row)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		return &ExportResult{Path: outPath, SHA256: hex.EncodeToString(sum[:]), Count: len(products), Format: format}, nil
	default:
		return nil, fmt.Errorf("format %q is not supported; use json or csv", format)
	}
}

func (s *Store) Doctor() map[string]any {
	registryDir := filepath.Join(s.DataDir, "registry", "cosing")
	files := map[string]any{}
	for _, name := range []string{"manifest.json", "ingredients.json", "annex_II.csv", "annex_III.csv", "annex_IV.csv", "annex_V.csv", "annex_VI.csv"} {
		path := filepath.Join(registryDir, name)
		if info, err := os.Stat(path); err == nil {
			files[name] = info.Size()
		} else {
			files[name] = nil
		}
	}
	return map[string]any{
		"data_dir":       s.DataDir,
		"local_dir":      s.LocalDir,
		"offline":        s.Offline,
		"registry":       s.Registry.Info(),
		"annexes":        s.Registry.AnnexSummary(),
		"registry_files": files,
		"products":       len(s.Products),
		"shelves":        len(s.Shelves),
		"generator":      GeneratorVersion,
	}
}
