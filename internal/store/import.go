package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jkalend/skincare_ingredients/internal/inci"
)

var (
	localProductKeyRE = regexp.MustCompile(`^local:[a-z0-9]+(?:[-_][a-z0-9]+)*$`)
	shelfNameRE       = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

type inventoryManifest struct {
	SchemaVersion int                `json:"schema_version"`
	Shelf         string             `json:"shelf,omitempty"`
	Products      []inventoryProduct `json:"products"`
}

type inventoryProduct struct {
	ProductKey    string           `json:"product_key"`
	Brand         string           `json:"brand"`
	Name          string           `json:"name"`
	Size          string           `json:"size,omitempty"`
	UnitsOwned    int              `json:"units_owned"`
	INCI          string           `json:"inci,omitempty"`
	PanelType     string           `json:"panel_type,omitempty"`
	VariantStatus string           `json:"variant_status,omitempty"`
	Notes         []string         `json:"notes,omitempty"`
	Source        *inventorySource `json:"source,omitempty"`
}

type inventorySource struct {
	Name          string   `json:"source"`
	URL           string   `json:"url"`
	FetchedAt     *string  `json:"fetched_at"`
	Method        string   `json:"method"`
	Confidence    float64  `json:"confidence"`
	RobotsAllowed *bool    `json:"robots_allowed,omitempty"`
	Notes         []string `json:"notes,omitempty"`
}

type SourceObservation struct {
	ID            string         `json:"id"`
	ProductKey    string         `json:"product_key"`
	Source        inci.SourceRef `json:"source"`
	Host          string         `json:"host"`
	RawSHA256     string         `json:"raw_sha256"`
	RawText       string         `json:"raw_text"`
	RobotsAllowed *bool          `json:"robots_allowed"`
	Notes         []string       `json:"notes"`
	Storage       string         `json:"storage"`
}

type ImportResult struct {
	Shelf       string `json:"shelf"`
	Products    int    `json:"products"`
	Units       int    `json:"units"`
	Ingredients int    `json:"ingredients"`
	Unresolved  int    `json:"unresolved"`
	Sources     int    `json:"sources"`
	DryRun      bool   `json:"dry_run"`
}

// ImportInventory validates a complete private inventory manifest before it
// writes anything. Imported products and source text are always stored below
// LocalDir, regardless of their source.
func (s *Store) ImportInventory(path, shelfName string, force, dryRun bool) (*ImportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read inventory manifest: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var manifest inventoryManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse inventory manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("parse inventory manifest: trailing data")
	}
	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("schema_version must be 1")
	}
	if len(manifest.Products) == 0 {
		return nil, fmt.Errorf("manifest has no products")
	}
	if shelfName == "" {
		shelfName = manifest.Shelf
	}
	if shelfName == "" {
		shelfName = "default"
	}
	if !shelfNameRE.MatchString(shelfName) {
		return nil, fmt.Errorf("shelf name must contain only letters, numbers, _ or -")
	}

	type preparedProduct struct {
		product     *inci.Product
		observation *SourceObservation
	}
	prepared := make([]preparedProduct, 0, len(manifest.Products))
	seenKeys := map[string]bool{}
	seenFiles := map[string]bool{}
	result := &ImportResult{Shelf: shelfName, Products: len(manifest.Products), DryRun: dryRun}
	for i, spec := range manifest.Products {
		if !localProductKeyRE.MatchString(spec.ProductKey) {
			return nil, fmt.Errorf("product %d: product_key must use local:<lowercase-slug>", i+1)
		}
		if seenKeys[spec.ProductKey] {
			return nil, fmt.Errorf("product %d: duplicate product_key %q", i+1, spec.ProductKey)
		}
		seenKeys[spec.ProductKey] = true
		productFile := fileKey(spec.ProductKey)
		if seenFiles[productFile] {
			return nil, fmt.Errorf("product %d: product_key collides with another local filename", i+1)
		}
		seenFiles[productFile] = true
		if strings.TrimSpace(spec.Brand) == "" || strings.TrimSpace(spec.Name) == "" {
			return nil, fmt.Errorf("product %q: brand and name are required", spec.ProductKey)
		}
		if spec.UnitsOwned < 1 {
			return nil, fmt.Errorf("product %q: units_owned must be at least 1", spec.ProductKey)
		}
		status := spec.VariantStatus
		if status == "" {
			status = "confirmed"
		}
		if status != "confirmed" && status != "uncertain" && status != "unresolved" {
			return nil, fmt.Errorf("product %q: variant_status must be confirmed, uncertain, or unresolved", spec.ProductKey)
		}

		product := &inci.Product{
			ProductKey:       spec.ProductKey,
			Brand:            strings.TrimSpace(spec.Brand),
			Name:             strings.TrimSpace(spec.Name),
			Size:             strings.TrimSpace(spec.Size),
			UnitsOwned:       spec.UnitsOwned,
			VariantStatus:    status,
			Notes:            cleanNotes(spec.Notes),
			Ingredients:      []inci.Line{},
			MayContain:       []inci.Line{},
			Unresolved:       []inci.Unresolved{},
			Trust:            "unverified",
			Sources:          []string{},
			Generated:        true,
			GeneratorVersion: GeneratorVersion,
		}
		panel := strings.TrimSpace(spec.INCI)
		if panel == "" {
			if status != "unresolved" || spec.Source != nil {
				return nil, fmt.Errorf("product %q: a missing INCI list requires variant_status unresolved and no source", spec.ProductKey)
			}
			if len(product.Notes) == 0 {
				return nil, fmt.Errorf("product %q: explain why the variant is unresolved in notes", spec.ProductKey)
			}
		} else {
			if spec.Source == nil {
				return nil, fmt.Errorf("product %q: source metadata is required for an INCI list", spec.ProductKey)
			}
			ref, host, err := sourceRef(spec.Source)
			if err != nil {
				return nil, fmt.Errorf("product %q: %w", spec.ProductKey, err)
			}
			if spec.Source.Name != "label-photo" && (spec.Source.RobotsAllowed == nil || !*spec.Source.RobotsAllowed) {
				return nil, fmt.Errorf("product %q: confirm robots permission for the source host before importing", spec.ProductKey)
			}
			if spec.Source.Name != "manufacturer" && spec.Source.Name != "retailer" && spec.Source.Name != "label-photo" {
				return nil, fmt.Errorf("product %q: source must be manufacturer, retailer, or label-photo", spec.ProductKey)
			}
			block, tail := inci.Trim(panel)
			if strings.TrimSpace(tail) != "" {
				return nil, fmt.Errorf("product %q: INCI text contains trailing non-ingredient text; trim it at the source", spec.ProductKey)
			}
			parsed := inci.Parse(block)
			if len(parsed) < 3 {
				return nil, fmt.Errorf("product %q: INCI list must contain at least three tokens", spec.ProductKey)
			}
			if spec.PanelType != "water-based" && spec.PanelType != "anhydrous" {
				return nil, fmt.Errorf("product %q: panel_type must be water-based or anhydrous", spec.ProductKey)
			}
			var declared, may []inci.Ingredient
			for _, item := range parsed {
				if item.Position == inci.MayContain {
					may = append(may, item)
				} else {
					declared = append(declared, item)
				}
			}
			product.Ingredients, _ = s.Registry.Resolver.ResolveAll(declared)
			product.MayContain, _ = s.Registry.Resolver.ResolveAll(may)
			_, product.Unresolved = s.Registry.Resolver.ResolveAll(parsed)
			product.Source = &ref
			product.Trust = "unverified" // one observation is never promoted to verified
			observation := newObservation(spec.ProductKey, ref, host, panel, spec.Source.RobotsAllowed, spec.Source.Notes)
			product.Sources = []string{observation.ID}
			result.Sources++
			result.Ingredients += len(product.Ingredients) + len(product.MayContain)
			result.Unresolved += len(product.Unresolved)
			prepared = append(prepared, preparedProduct{product: product, observation: observation})
		}
		if panel == "" {
			prepared = append(prepared, preparedProduct{product: product})
		}
		result.Units += spec.UnitsOwned
	}

	// Refuse an accidental overwrite before the first write. Byte-equivalent
	// records are safe to re-import and produce no changes.
	for _, item := range prepared {
		path := filepath.Join(s.LocalDir, "products", fileKey(item.product.ProductKey))
		if err := checkExisting(path, item.product, force); err != nil {
			return nil, err
		}
		if item.observation != nil {
			path = filepath.Join(s.LocalDir, "sources", strings.TrimPrefix(item.observation.ID, "sha256:")+".json")
			if err := checkExisting(path, item.observation, force); err != nil {
				return nil, err
			}
		}
	}
	shelf := s.Shelf(shelfName)
	for _, item := range prepared {
		shelf.Items = append(shelf.Items, item.product.ProductKey)
	}
	shelf.Items = unique(shelf.Items)
	sort.Strings(shelf.Items)
	if dryRun {
		return result, nil
	}
	for _, item := range prepared {
		if err := s.SaveProduct(item.product); err != nil {
			return nil, fmt.Errorf("write %s: %w", item.product.ProductKey, err)
		}
		if item.observation != nil {
			path := filepath.Join(s.LocalDir, "sources", strings.TrimPrefix(item.observation.ID, "sha256:")+".json")
			if err := writeJSON(path, item.observation); err != nil {
				return nil, fmt.Errorf("write source for %s: %w", item.product.ProductKey, err)
			}
		}
	}
	if err := s.SaveShelf(shelf); err != nil {
		return nil, fmt.Errorf("write shelf: %w", err)
	}
	return result, nil
}

func sourceRef(source *inventorySource) (inci.SourceRef, string, error) {
	if source == nil {
		return inci.SourceRef{}, "", fmt.Errorf("source metadata is required")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(source.URL))
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil {
		return inci.SourceRef{}, "", fmt.Errorf("source URL must be a valid HTTPS URL without credentials")
	}
	if source.FetchedAt != nil {
		if _, err := time.Parse(time.RFC3339, *source.FetchedAt); err != nil {
			return inci.SourceRef{}, "", fmt.Errorf("source fetched_at must be RFC3339 or null when the exact time is unknown")
		}
	}
	if strings.TrimSpace(source.Method) == "" || source.Confidence <= 0 || source.Confidence > 1 {
		return inci.SourceRef{}, "", fmt.Errorf("source method is required and confidence must be in (0, 1]")
	}
	return inci.SourceRef{
		Source:     source.Name,
		URL:        strings.TrimSpace(source.URL),
		FetchedAt:  source.FetchedAt,
		Method:     strings.TrimSpace(source.Method),
		Confidence: source.Confidence,
	}, strings.ToLower(parsedURL.Hostname()), nil
}

func newObservation(productKey string, source inci.SourceRef, host, raw string, robotsAllowed *bool, notes []string) *SourceObservation {
	contentHash := sha256.Sum256([]byte(raw))
	identity := sha256.Sum256([]byte(productKey + "\x00" + source.URL + "\x00" + raw))
	return &SourceObservation{
		ID:            "sha256:" + hex.EncodeToString(identity[:]),
		ProductKey:    productKey,
		Source:        source,
		Host:          host,
		RawSHA256:     hex.EncodeToString(contentHash[:]),
		RawText:       raw,
		RobotsAllowed: robotsAllowed,
		Notes:         cleanNotes(notes),
		Storage:       "local-only",
	}
}

func cleanNotes(notes []string) []string {
	out := make([]string, 0, len(notes))
	for _, note := range notes {
		if note = strings.TrimSpace(note); note != "" {
			out = append(out, note)
		}
	}
	return out
}

func checkExisting(path string, value any, force bool) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	current, err := canonicalJSON(data)
	if err != nil {
		return fmt.Errorf("existing file %s is not valid JSON: %w", path, err)
	}
	wantedBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	wanted, err := canonicalJSON(wantedBytes)
	if err != nil {
		return err
	}
	if current == wanted || force {
		return nil
	}
	return fmt.Errorf("existing record differs at %s; pass --force to replace it", path)
}

func canonicalJSON(data []byte) (string, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	return string(canonical), err
}
