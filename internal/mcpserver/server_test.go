package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jkalend/skincare_ingredients/internal/service"
	"github.com/jkalend/skincare_ingredients/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsAndAnalyze(t *testing.T) {
	svc := service.New(testutil.Store(t))
	server := New(svc, true)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "analyze_inci") || !strings.Contains(joined, "lookup_product") {
		t.Fatalf("read tools = %v", names)
	}
	if strings.Contains(joined, "shelf_add") {
		t.Fatalf("read-only server exposed a write tool: %v", names)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "analyze_inci",
		Arguments: map[string]any{"text": "Aqua, Glycerin, Cyclomethicone"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "AQUA") || !strings.Contains(string(encoded), "1388") {
		t.Fatalf("structured result = %s", encoded)
	}
}

func TestHTTPTransport(t *testing.T) {
	svc := service.New(testutil.Store(t))
	httpServer := httptest.NewServer(HTTPHandler(svc, true, ""))
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "read_only") {
		t.Fatalf("healthz: %d %s", resp.StatusCode, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "analyze_inci",
		Arguments: map[string]any{"text": "Aqua, Glycerin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(encoded), "AQUA") {
		t.Fatalf("HTTP result = %s", encoded)
	}
}

func TestHTTPToken(t *testing.T) {
	svc := service.New(testutil.Store(t))
	httpServer := httptest.NewServer(HTTPHandler(svc, true, "secret"))
	defer httpServer.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`
	req, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without token: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPost, httpServer.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("with token: %d", resp.StatusCode)
	}
}

func TestWriteToolsAvailableWithoutReadOnly(t *testing.T) {
	svc := service.New(testutil.Store(t))
	server := New(svc, false)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"shelf_add", "shelf_rm", "shelf_list"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
}
