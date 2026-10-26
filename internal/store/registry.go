package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jkalend/skincare_ingredients/internal/inci"
)

type Registry struct {
	Manifest inci.Manifest
	Records  []inci.IngredientRecord
	Annex    []inci.AnnexEntry
	Resolver *inci.Resolver
}

// LoadRegistry reads the snapshot written by scripts/fetch_registry.py.
func LoadRegistry(dir string) (*Registry, error) {
	manifestBytes, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read registry manifest: %w", err)
	}
	var manifest inci.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("parse registry manifest: %w", err)
	}
	recordBytes, err := os.ReadFile(filepath.Join(dir, "ingredients.json"))
	if err != nil {
		return nil, fmt.Errorf("read registry ingredients: %w", err)
	}
	var records []inci.IngredientRecord
	if err := json.Unmarshal(recordBytes, &records); err != nil {
		return nil, fmt.Errorf("parse registry ingredients: %w", err)
	}

	var entries []inci.AnnexEntry
	for _, annex := range []string{"II", "III", "IV", "V", "VI"} {
		data, err := os.ReadFile(filepath.Join(dir, "annex_"+annex+".csv"))
		if err != nil {
			return nil, fmt.Errorf("read annex %s: %w", annex, err)
		}
		if expected := manifest.Annexes[annex].ContentSHA256; isSHA256(expected) {
			if got := contentSHA256(data); got != expected {
				return nil, fmt.Errorf("annex %s content hash mismatch: got %s, manifest %s", annex, got, expected)
			}
		}
		parsed, err := inci.ReadAnnex(annex, data)
		if err != nil {
			return nil, err
		}
		entries = append(entries, parsed...)
	}

	return &Registry{
		Manifest: manifest,
		Records:  records,
		Annex:    entries,
		Resolver: inci.NewResolver(records, entries),
	}, nil
}

func (r *Registry) Info() inci.RegistryInfo {
	info := inci.RegistryInfo{
		FetchedAt:    r.Manifest.FetchedAt,
		Ingredients:  len(r.Records),
		AnnexEntries: len(r.Annex),
	}
	// All annexes share the Commission's last-update date; use Annex II as the
	// cited register and its stripped content hash as the identity.
	if annex, ok := r.Manifest.Annexes["II"]; ok {
		info.CosIngAnnexes = annex.LastUpdate
		info.SHA256 = annex.ContentSHA256
	}
	return info
}

func (r *Registry) AnnexSummary() map[string]any {
	out := map[string]any{}
	for name, annex := range r.Manifest.Annexes {
		out[name] = map[string]any{
			"last_update":    annex.LastUpdate,
			"file_creation":  annex.FileCreationDate,
			"content_sha256": annex.ContentSHA256,
			"data_rows":      annex.DataRows,
			"source_url":     annex.SourceURL,
		}
	}
	out["mirror"] = map[string]any{
		"source":           r.Manifest.Mirror.Source,
		"glossary_version": r.Manifest.Mirror.GlossaryVersion,
		"glossary_count":   r.Manifest.Mirror.GlossaryCount,
		"ingredient_count": r.Manifest.Mirror.IngredientCount,
		"function_count":   r.Manifest.Mirror.FunctionCount,
		"fetched_at":       r.Manifest.Mirror.FetchedAt,
	}
	return out
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func contentSHA256(data []byte) string {
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 2 {
		data = bytes.Join(lines[2:], nil)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
