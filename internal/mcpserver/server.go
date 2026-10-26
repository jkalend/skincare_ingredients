package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jkalend/skincare_ingredients/internal/inci"
	"github.com/jkalend/skincare_ingredients/internal/service"
	"github.com/jkalend/skincare_ingredients/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const caveat = "Ingredient order is not concentration; ingredients below 1% may be listed in any order. Annex matches are citations, not safety or compliance verdicts."

type ToolOutput struct {
	OK         bool                `json:"ok"`
	Exit       int                 `json:"exit"`
	Data       any                 `json:"data"`
	Warnings   []service.Warning   `json:"warnings"`
	WorkOrders []service.WorkOrder `json:"work_orders"`
	Registry   inci.RegistryInfo   `json:"registry"`
	Next       []string            `json:"next"`
}

func output(svc *service.Service, resp *service.Response) ToolOutput {
	if resp == nil {
		return ToolOutput{OK: false, Exit: 10, Data: map[string]any{"error": "empty response"}, Warnings: []service.Warning{}, WorkOrders: []service.WorkOrder{}, Registry: svc.Registry(), Next: []string{}}
	}
	warnings := resp.Warnings
	if warnings == nil {
		warnings = []service.Warning{}
	}
	orders := resp.WorkOrders
	if orders == nil {
		orders = []service.WorkOrder{}
	}
	next := resp.Next
	if next == nil {
		next = []string{}
	}
	return ToolOutput{
		OK:         resp.Exit < 2,
		Exit:       resp.Exit,
		Data:       resp.Data,
		Warnings:   warnings,
		WorkOrders: orders,
		Registry:   svc.Registry(),
		Next:       next,
	}
}

type lookupInput struct {
	Barcode string `json:"barcode" jsonschema:"8-14 digit product barcode"`
}

type searchInput struct {
	Query string `json:"query" jsonschema:"product or brand substring"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum results (default 10)"`
}

type textInput struct {
	Text string `json:"text" jsonschema:"ingredient list or full label text"`
}

type ingredientInput struct {
	Name string `json:"name" jsonschema:"printed or canonical ingredient name"`
}

type shelfInput struct {
	Shelf string `json:"shelf,omitempty" jsonschema:"shelf name (default: default)"`
}

type shelfAddInput struct {
	Barcode string `json:"barcode" jsonschema:"8-14 digit product barcode"`
	Shelf   string `json:"shelf,omitempty" jsonschema:"shelf name (default: default)"`
}

type shelfRemoveInput struct {
	Barcode string `json:"barcode" jsonschema:"8-14 digit product barcode"`
	Shelf   string `json:"shelf,omitempty" jsonschema:"shelf name (default: default)"`
}

func New(svc *service.Service, readOnly bool) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "skincare-ingredients",
		Version: store.GeneratorVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_product",
		Description: "Find a product by barcode from the local cache, then Open Beauty Facts. " + caveat,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in lookupInput) (*mcp.CallToolResult, ToolOutput, error) {
		return nil, output(svc, svc.Lookup(ctx, in.Barcode, false)), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_products",
		Description: "Search cached products by brand, name or barcode. " + caveat,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, ToolOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 10
		}
		return nil, output(svc, svc.Search(in.Query, limit)), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "parse_ingredients",
		Description: "Parse label text into an ordered ingredient list and unresolved tokens. " + caveat,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in textInput) (*mcp.CallToolResult, ToolOutput, error) {
		return nil, output(svc, svc.Analyze(in.Text)), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_inci",
		Description: "Parse label text, resolve each token to a CosIng name, and attach function and annex citations. " + caveat,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in textInput) (*mcp.CallToolResult, ToolOutput, error) {
		return nil, output(svc, svc.Analyze(in.Text)), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ingredient",
		Description: "Get one canonical ingredient record and its annex matches by exact folded name. " + caveat,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ingredientInput) (*mcp.CallToolResult, ToolOutput, error) {
		return nil, output(svc, svc.Ingredient(in.Name)), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "doctor",
		Description: "Report registry version, data counts, and whether the local data directory is complete.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ToolOutput, error) {
		return nil, output(svc, svc.Doctor()), nil
	})

	if !readOnly {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "shelf_list",
			Description: "List the products on a shelf. The default shelf lives in the private local overlay. " + caveat,
		}, func(_ context.Context, _ *mcp.CallToolRequest, in shelfInput) (*mcp.CallToolResult, ToolOutput, error) {
			return nil, output(svc, svc.ShelfList(in.Shelf)), nil
		})
		mcp.AddTool(server, &mcp.Tool{
			Name:        "shelf_add",
			Description: "Add a product barcode to a shelf in the private local overlay. Write tool. " + caveat,
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in shelfAddInput) (*mcp.CallToolResult, ToolOutput, error) {
			return nil, output(svc, svc.ShelfAdd(in.Shelf, in.Barcode)), nil
		})
		mcp.AddTool(server, &mcp.Tool{
			Name:        "shelf_rm",
			Description: "Remove a product barcode from a shelf in the private local overlay. Write tool. " + caveat,
		}, func(_ context.Context, _ *mcp.CallToolRequest, in shelfRemoveInput) (*mcp.CallToolResult, ToolOutput, error) {
			return nil, output(svc, svc.ShelfRemove(in.Shelf, in.Barcode)), nil
		})
	}

	return server
}

func RunStdio(ctx context.Context, svc *service.Service, readOnly bool) error {
	server := New(svc, readOnly)
	return server.Run(ctx, &mcp.StdioTransport{})
}

func HTTPHandler(svc *service.Service, readOnly bool, token string) http.Handler {
	server := New(svc, readOnly)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:      true,
		JSONResponse:   true,
		SessionTimeout: 5 * time.Minute,
	})
	protected := func(next http.Handler) http.Handler {
		if token == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+token {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", protected(mcpHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"registry":  svc.Registry(),
			"read_only": readOnly,
		})
	})
	return mux
}

func RunHTTP(ctx context.Context, svc *service.Service, addr string, readOnly bool, token string) error {
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: HTTPHandler(svc, readOnly, token)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
