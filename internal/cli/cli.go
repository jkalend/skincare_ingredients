package cli

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jkalend/skincare_ingredients/internal/inci"
	"github.com/jkalend/skincare_ingredients/internal/mcpserver"
	"github.com/jkalend/skincare_ingredients/internal/service"
	"github.com/jkalend/skincare_ingredients/internal/store"
)

const usage = `skincare — free, agent-native skincare ingredient index

Usage:
  skincare [global flags] <command> [command flags] [args]

Commands:
  lookup <barcode>              cached product, then Open Beauty Facts
  search <query>                search cached products
  parse                         label text -> ordered tokens
  analyze                       label text -> canonical names, functions, annex
  resolve <token...>            resolve printed names
  ingredient <name>             one canonical ingredient + annex
  shelf add <barcode>           add to a shelf
  shelf ls                      list a shelf
  shelf rm <barcode>            remove from a shelf
  shelf report                  duplicates, functions, annex matches
  export                        export a shelf (json|csv)
  doctor                        registry/data diagnostics
  mcp                           stdio or HTTP MCP server
  corpus-check                  dev: parse the OBF CSV and report counts
  version                       version

Global flags must come before the command. "parse --text" is the input text,
not the human-output mode.

Global flags (before the command):
  --data-dir DIR    data directory (default data)
  --local-dir DIR   private overlay (default local)
  --cache-dir DIR   cache directory (default cache)
  --offline         no network; cached data only
  --refresh         refetch a product even if cached
  --text            human-readable output instead of JSON
  --timeout DUR     network timeout (default 20s)

Examples:
  skincare lookup 1000034026006
  skincare parse --text "Aqua, Glycerin, Niacinamide"
  skincare analyze --file label.txt
  skincare shelf add --shelf default 1000034026006
  skincare export --format csv --shelf default
  skincare mcp --transport stdio
  skincare mcp --transport http --addr :8080 --read-only

Exit codes: 0 ok, 1 incomplete, 2 usage, 3 not found, 4 needs input,
5 policy block, 10 internal error.
`

type globals struct {
	DataDir  string
	LocalDir string
	CacheDir string
	Offline  bool
	Refresh  bool
	Text     bool
	Timeout  time.Duration
}

func mainGlobals() globals {
	return globals{DataDir: "data", LocalDir: "local", CacheDir: "cache", Timeout: 20 * time.Second}
}

func splitGlobals(args []string) (globals, []string, error) {
	g := mainGlobals()
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			rest = append(rest, args[i:]...)
			break
		}
		name, value, hasValue := strings.Cut(arg, "=")
		take := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}
			i++
			return args[i], nil
		}
		switch name {
		case "-h", "--help":
			return g, []string{"help"}, nil
		case "--data-dir":
			v, err := take()
			if err != nil {
				return g, nil, err
			}
			g.DataDir = v
		case "--local-dir":
			v, err := take()
			if err != nil {
				return g, nil, err
			}
			g.LocalDir = v
		case "--cache-dir":
			v, err := take()
			if err != nil {
				return g, nil, err
			}
			g.CacheDir = v
		case "--timeout":
			v, err := take()
			if err != nil {
				return g, nil, err
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				seconds, convErr := strconv.Atoi(v)
				if convErr != nil {
					return g, nil, fmt.Errorf("invalid --timeout %q", v)
				}
				d = time.Duration(seconds) * time.Second
			}
			g.Timeout = d
		case "--offline":
			g.Offline = true
		case "--refresh":
			g.Refresh = true
		case "--text":
			g.Text = true
		case "--json":
			g.Text = false
		case "--seed", "--wait":
			if !hasValue && name == "--seed" {
				_, _ = take()
			}
		default:
			return g, nil, fmt.Errorf("unknown global flag %s", name)
		}
	}
	return g, rest, nil
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	g, rest, err := splitGlobals(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	command := rest[0]
	commandArgs := rest[1:]

	if command == "version" {
		return emit(g.Text, stdout, nil, &service.Response{
			Data: map[string]any{"version": store.GeneratorVersion, "language": "go"},
			Exit: 0, Warnings: []service.Warning{}, WorkOrders: []service.WorkOrder{}, Next: []string{},
		})
	}
	if command == "corpus-check" {
		return corpusCheck(commandArgs, stdin, stdout, stderr)
	}
	if command == "mcp" {
		return runMCP(g, commandArgs, stderr)
	}

	svc, err := newService(g)
	if err != nil {
		return emit(g.Text, stdout, nil, &service.Response{
			Data: map[string]any{"error": err.Error(), "hint": "run scripts/fetch_registry.py, or check --data-dir"},
			Exit: 10, Warnings: []service.Warning{}, WorkOrders: []service.WorkOrder{}, Next: []string{},
		})
	}

	switch command {
	case "lookup":
		if len(commandArgs) != 1 {
			return usageError(stderr, "lookup needs one barcode")
		}
		return emit(g.Text, stdout, svc, svc.Lookup(context.Background(), commandArgs[0], g.Refresh))
	case "search":
		fs := flag.NewFlagSet("search", flag.ContinueOnError)
		fs.SetOutput(stderr)
		limit := fs.Int("limit", 10, "maximum results")
		if err := fs.Parse(commandArgs); err != nil {
			return 2
		}
		query := strings.Join(fs.Args(), " ")
		if query == "" {
			return usageError(stderr, "search needs a query")
		}
		return emit(g.Text, stdout, svc, svc.Search(query, *limit))
	case "parse", "analyze":
		fs := flag.NewFlagSet(command, flag.ContinueOnError)
		fs.SetOutput(stderr)
		text := fs.String("text", "", "ingredient text")
		file := fs.String("file", "", "file containing label text")
		product := fs.String("product", "", "stored product key or barcode (analyze only)")
		if err := fs.Parse(commandArgs); err != nil {
			return 2
		}
		if command == "analyze" && *product != "" {
			stored := svc.Store.ProductByKey(*product)
			if stored == nil {
				stored = svc.Store.ProductByBarcode(*product)
			}
			if stored == nil {
				return emit(g.Text, stdout, svc, &service.Response{Data: map[string]any{"status": "not_found", "product": *product}, Exit: 3, Warnings: []service.Warning{}, WorkOrders: []service.WorkOrder{}, Next: []string{}})
			}
			exit := 0
			if len(stored.Unresolved) > 0 || stored.Trust != "verified" && stored.Trust != "manual" {
				exit = 1
			}
			return emit(g.Text, stdout, svc, &service.Response{Data: stored, Exit: exit, Warnings: []service.Warning{}, WorkOrders: []service.WorkOrder{}, Next: []string{}})
		}
		input, err := readInput(*text, *file, stdin)
		if err != nil {
			return usageError(stderr, err.Error())
		}
		return emit(g.Text, stdout, svc, svc.Analyze(input))
	case "resolve":
		if len(commandArgs) == 0 {
			return usageError(stderr, "resolve needs at least one token")
		}
		return emit(g.Text, stdout, svc, svc.Resolve(commandArgs))
	case "ingredient":
		name := strings.Join(commandArgs, " ")
		if name == "" {
			return usageError(stderr, "ingredient needs a name")
		}
		return emit(g.Text, stdout, svc, svc.Ingredient(name))
	case "shelf":
		return runShelf(g, svc, commandArgs, stdout, stderr)
	case "export":
		return runExport(g, svc, commandArgs, stdout, stderr)
	case "doctor":
		return emit(g.Text, stdout, svc, svc.Doctor())
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usage)
		return 2
	}
}

func newService(g globals) (*service.Service, error) {
	s, err := store.Open(store.Options{
		DataDir:  g.DataDir,
		LocalDir: g.LocalDir,
		CacheDir: g.CacheDir,
		Offline:  g.Offline,
		Timeout:  g.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return service.New(s), nil
}

func usageError(stderr io.Writer, message string) int {
	fmt.Fprintln(stderr, message)
	return 2
}

func readInput(text, file string, stdin io.Reader) (string, error) {
	if text != "" && file != "" {
		return "", errors.New("use --text or --file, not both")
	}
	if text != "" {
		return text, nil
	}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if stdin == nil {
		return "", errors.New("no input; use --text, --file, or stdin")
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 2<<20))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", errors.New("no input; use --text, --file, or stdin")
	}
	return string(data), nil
}

func runShelf(g globals, svc *service.Service, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "ls" {
		shelf := ""
		if len(args) > 0 {
			args = args[1:]
		}
		fs := flag.NewFlagSet("shelf ls", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fs.StringVar(&shelf, "shelf", "default", "shelf name")
		_ = fs.Parse(args)
		return emit(g.Text, stdout, svc, svc.ShelfList(shelf))
	}
	sub := args[0]
	args = args[1:]
	switch sub {
	case "add", "rm":
		fs := flag.NewFlagSet("shelf "+sub, flag.ContinueOnError)
		fs.SetOutput(stderr)
		shelf := fs.String("shelf", "default", "shelf name")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		positional := fs.Args()
		if len(positional) != 1 {
			return usageError(stderr, "shelf "+sub+" needs one barcode")
		}
		if sub == "add" {
			return emit(g.Text, stdout, svc, svc.ShelfAdd(*shelf, positional[0]))
		}
		return emit(g.Text, stdout, svc, svc.ShelfRemove(*shelf, positional[0]))
	case "report":
		fs := flag.NewFlagSet("shelf report", flag.ContinueOnError)
		fs.SetOutput(stderr)
		shelf := fs.String("shelf", "default", "shelf name")
		_ = fs.Parse(args)
		return emit(g.Text, stdout, svc, svc.ShelfReport(*shelf))
	default:
		return usageError(stderr, "shelf needs add, ls, rm or report")
	}
}

func runExport(g globals, svc *service.Service, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "json", "json or csv")
	out := fs.String("out", "", "output path")
	shelf := fs.String("shelf", "default", "shelf name")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return emit(g.Text, stdout, svc, svc.Export(*shelf, *format, *out))
}

func runMCP(g globals, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	transport := fs.String("transport", "stdio", "stdio or http")
	addr := fs.String("addr", ":8080", "HTTP listen address")
	readOnly := fs.Bool("read-only", false, "expose read tools only; also hides the private local overlay")
	token := fs.String("token", os.Getenv("SKINCARE_MCP_TOKEN"), "optional bearer token for HTTP MCP")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	st, err := store.Open(store.Options{
		DataDir:    g.DataDir,
		LocalDir:   g.LocalDir,
		CacheDir:   g.CacheDir,
		Offline:    g.Offline,
		Timeout:    g.Timeout,
		PublicOnly: *readOnly,
	})
	if err != nil {
		fmt.Fprintln(stderr, "open data:", err)
		return 10
	}
	svc := service.New(st)
	ctx := context.Background()
	switch *transport {
	case "stdio":
		fmt.Fprintln(stderr, "mcp: stdio ready (logs on stderr)")
		if err := mcpserver.RunStdio(ctx, svc, *readOnly); err != nil {
			fmt.Fprintln(stderr, "mcp:", err)
			return 10
		}
		return 0
	case "http":
		if !*readOnly {
			return usageError(stderr, "HTTP MCP is read-only in v1; pass --read-only (use stdio for shelf writes)")
		}
		fmt.Fprintf(stderr, "mcp: HTTP listening on %s/mcp (read_only=%v, public_only=%v, token=%v)\n", *addr, *readOnly, *readOnly, *token != "")
		if err := mcpserver.RunHTTP(ctx, svc, *addr, *readOnly, *token); err != nil {
			fmt.Fprintln(stderr, "mcp:", err)
			return 10
		}
		return 0
	default:
		return usageError(stderr, "mcp --transport must be stdio or http")
	}
}

func emit(textMode bool, stdout io.Writer, svc *service.Service, resp *service.Response) int {
	if resp == nil {
		resp = &service.Response{Data: map[string]any{"error": "empty response"}, Exit: 10}
	}
	if resp.Warnings == nil {
		resp.Warnings = []service.Warning{}
	}
	if resp.WorkOrders == nil {
		resp.WorkOrders = []service.WorkOrder{}
	}
	if resp.Next == nil {
		resp.Next = []string{}
	}
	registry := inci.RegistryInfo{}
	if svc != nil {
		registry = svc.Registry()
	}
	ok := resp.Exit == 0 || resp.Exit == 1 || resp.Exit == 4
	if textMode {
		renderText(stdout, resp.Data)
		if len(resp.Warnings) > 0 {
			for _, warning := range resp.Warnings {
				fmt.Fprintf(stdout, "warning: %s: %s\n", warning.Code, warning.Message)
			}
		}
		return resp.Exit
	}
	envelope := map[string]any{
		"ok":          ok,
		"exit":        resp.Exit,
		"data":        resp.Data,
		"warnings":    resp.Warnings,
		"work_orders": resp.WorkOrders,
		"registry":    registry,
		"next":        resp.Next,
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(envelope); err != nil {
		fmt.Fprintln(os.Stderr, "encode response:", err)
		return 10
	}
	return resp.Exit
}

func renderText(w io.Writer, data any) {
	switch value := data.(type) {
	case *service.Analysis:
		for _, line := range value.Ingredients {
			canonical := "UNRESOLVED"
			if line.Canonical != nil {
				canonical = *line.Canonical
			}
			fmt.Fprintf(w, "%d. %s -> %s", line.Position, line.Printed, canonical)
			if len(line.Functions) > 0 {
				fmt.Fprintf(w, " [%s]", strings.Join(line.Functions, ", "))
			}
			if line.Annex != nil {
				for _, match := range line.Annex.Matches {
					fmt.Fprintf(w, " (Annex %s #%s)", match.Annex, match.Reference)
				}
			}
			fmt.Fprintln(w)
		}
		if len(value.MayContain) > 0 {
			fmt.Fprintln(w, "May contain:")
			for _, line := range value.MayContain {
				fmt.Fprintf(w, "  %s\n", line.Printed)
			}
		}
		if len(value.Unresolved) > 0 {
			fmt.Fprintf(w, "Unresolved: %d token(s)\n", len(value.Unresolved))
		}
	case *inci.Product:
		fmt.Fprintf(w, "%s %s\n", value.Brand, value.Name)
		if value.Barcode != "" {
			fmt.Fprintf(w, "barcode: %s  trust: %s\n", value.Barcode, value.Trust)
		}
		for _, line := range value.Ingredients {
			canonical := "UNRESOLVED"
			if line.Canonical != nil {
				canonical = *line.Canonical
			}
			fmt.Fprintf(w, "%d. %s -> %s\n", line.Position, line.Printed, canonical)
		}
	default:
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(value)
	}
}

func corpusCheck(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("corpus-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("csv", "", "OBF products CSV or CSV.GZ")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		return usageError(stderr, "corpus-check needs --csv")
	}
	start := time.Now()
	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer file.Close()
	var reader io.Reader = file
	if strings.HasSuffix(strings.ToLower(*path), ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		defer gz.Close()
		reader = gz
	}
	csvReader := csv.NewReader(reader)
	csvReader.Comma = '\t'
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	header, err := csvReader.Read()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	ingredientIndex := -1
	for i, name := range header {
		if name == "ingredients_text" {
			ingredientIndex = i
			break
		}
	}
	if ingredientIndex < 0 {
		fmt.Fprintln(stderr, "csv has no ingredients_text column")
		return 2
	}
	records, selected, parsed, tokens := 0, 0, 0, 0
	for {
		row, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		records++
		if len(row) <= ingredientIndex || len(strings.TrimSpace(row[ingredientIndex])) < 50 {
			continue
		}
		selected++
		block, _ := inci.Trim(row[ingredientIndex])
		items := inci.Parse(block)
		if len(items) > 0 {
			parsed++
			tokens += len(items)
		}
	}
	rate := 0.0
	if selected > 0 {
		rate = float64(parsed) / float64(selected) * 100
	}
	result := map[string]any{
		"records":        records,
		"selected":       selected,
		"parsed":         parsed,
		"parse_rate_pct": rate,
		"ingredients":    tokens,
		"seconds":        time.Since(start).Seconds(),
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
	return 0
}
