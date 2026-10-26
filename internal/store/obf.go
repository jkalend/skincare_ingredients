package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("product not found")
	ErrNoPanel  = errors.New("product has no ingredient panel")
)

type OBFProduct struct {
	Barcode     string
	Name        string
	Brand       string
	Size        string
	Ingredients string
	URL         string
	FetchedAt   string
}

type obfResponse struct {
	Status  int `json:"status"`
	Product *struct {
		Code            string `json:"code"`
		ProductName     string `json:"product_name"`
		Brands          string `json:"brands"`
		Quantity        string `json:"quantity"`
		IngredientsText string `json:"ingredients_text"`
	} `json:"product"`
}

func (s *Store) FetchOBF(ctx context.Context, barcode string) (*OBFProduct, error) {
	if s.Offline {
		return nil, fmt.Errorf("offline mode: no network lookup")
	}
	endpoint := "https://world.openbeautyfacts.org/api/v2/product/" +
		url.PathEscape(barcode) + ".json?fields=code,product_name,brands,quantity,ingredients_text"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.UserAgent)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: s.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open beauty facts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("open beauty facts: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded obfResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("open beauty facts: %w", err)
	}
	if decoded.Status == 0 || decoded.Product == nil {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(decoded.Product.IngredientsText) == "" {
		return nil, ErrNoPanel
	}
	return &OBFProduct{
		Barcode:     barcode,
		Name:        strings.TrimSpace(decoded.Product.ProductName),
		Brand:       strings.TrimSpace(decoded.Product.Brands),
		Size:        strings.TrimSpace(decoded.Product.Quantity),
		Ingredients: decoded.Product.IngredientsText,
		URL:         "https://world.openbeautyfacts.org/product/" + url.PathEscape(barcode),
		FetchedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}
