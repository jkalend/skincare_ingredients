package inci

// SourceRef is provenance for a stored value. Every record that carries data
// from a network source must be able to answer: where did this come from, when,
// by what method, and how sure are we?
type SourceRef struct {
	Source     string  `json:"source"`
	URL        string  `json:"url,omitempty"`
	FetchedAt  string  `json:"fetched_at,omitempty"`
	Method     string  `json:"method,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type Position string

const (
	Declared   Position = "declared"
	MayContain Position = "may-contain"
)

// Alias is another form of a printed name worth resolving. Kind is "whole"
// when the form is still the ingredient's name (brackets removed) and "part"
// when it is only a fragment of what was printed.
type Alias struct {
	Text string
	Kind string
}

// Ingredient is one token parsed from a label, before resolution.
type Ingredient struct {
	Printed     string   `json:"printed"`
	Normalised  string   `json:"normalised"`
	Index       int      `json:"position"`
	Position    Position `json:"section"`
	DeclaredPct *float64 `json:"declared_pct,omitempty"`
	Aliases     []Alias  `json:"-"`
}

type AnnexMatch struct {
	Annex            string `json:"annex"`
	Reference        string `json:"reference"`
	ChemicalName     string `json:"chemical_name,omitempty"`
	MatchedName      string `json:"matched_name,omitempty"`
	NameSource       string `json:"name_source,omitempty"`
	ProductTypes     string `json:"product_types,omitempty"`
	MaxConcentration string `json:"max_concentration,omitempty"`
	Warnings         string `json:"warnings,omitempty"`
	Qualifier        string `json:"qualifier,omitempty"`
	NanoOnly         bool   `json:"nano_only,omitempty"`
}

type AnnexStatus struct {
	Matches []AnnexMatch `json:"matches"`
}

// Resolution is one printed token after the deterministic chain has run.
type Resolution struct {
	Printed    string       `json:"printed"`
	Canonical  *string      `json:"canonical"`
	Functions  []string     `json:"functions"`
	Annex      *AnnexStatus `json:"annex"`
	Method     string       `json:"method"`
	Confidence float64      `json:"confidence"`
}

type NormalizedName struct {
	Name   string
	Source string // glossary | identified | colour-index
}

type AnnexEntry struct {
	Annex            string
	Reference        string
	ChemicalName     string
	Names            []string
	NormalisedNames  []NormalizedName
	Wording          string
	ProductTypes     string
	MaxConcentration string
	UpdateDate       string
	Qualifier        string
	NanoOnly         bool
}

type IngredientRecord struct {
	INCIName      string    `json:"inci_name"`
	INNName       *string   `json:"inn_name"`
	PhEurName     *string   `json:"ph_eur_name"`
	CASNumber     *string   `json:"cas_number"`
	ECNumber      *string   `json:"ec_number"`
	Description   *string   `json:"description"`
	Restriction   *string   `json:"restriction"`
	RefNumber     *string   `json:"ref_number"`
	Functions     []string  `json:"functions"`
	GlossaryEntry *int      `json:"glossary_entry,omitempty"`
	Source        SourceRef `json:"source"`
}

type Line struct {
	Position    int          `json:"position"`
	Printed     string       `json:"printed"`
	Canonical   *string      `json:"canonical"`
	Functions   []string     `json:"functions"`
	Annex       *AnnexStatus `json:"annex"`
	DeclaredPct *float64     `json:"declared_pct"`
	Method      string       `json:"method"`
	Confidence  float64      `json:"confidence"`
}

type Unresolved struct {
	Printed  string   `json:"printed"`
	Position int      `json:"position"`
	Section  Position `json:"section"`
	Reason   string   `json:"reason"`
}

type Product struct {
	ProductKey       string       `json:"product_key"`
	Barcode          string       `json:"barcode,omitempty"`
	Brand            string       `json:"brand,omitempty"`
	Name             string       `json:"name,omitempty"`
	Size             string       `json:"size,omitempty"`
	Ingredients      []Line       `json:"ingredients"`
	MayContain       []Line       `json:"may_contain"`
	Unresolved       []Unresolved `json:"unresolved"`
	Trust            string       `json:"trust"`
	Sources          []string     `json:"sources"`
	Source           *SourceRef   `json:"source,omitempty"`
	Generated        bool         `json:"generated"`
	GeneratorVersion string       `json:"generator_version"`
}

type RegistryInfo struct {
	CosIngAnnexes string `json:"cosing_annexes,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	FetchedAt     string `json:"fetched_at,omitempty"`
	Ingredients   int    `json:"ingredients,omitempty"`
	AnnexEntries  int    `json:"annex_entries,omitempty"`
}

type ManifestAnnex struct {
	SourceURL        string `json:"source_url"`
	FetchedAt        string `json:"fetched_at"`
	RawSHA256        string `json:"raw_sha256"`
	RawBytes         int64  `json:"raw_bytes"`
	ContentSHA256    string `json:"content_sha256"`
	FileCreationDate string `json:"file_creation_date"`
	LastUpdate       string `json:"last_update"`
	DataRows         int    `json:"data_rows"`
}

type ManifestMirror struct {
	Source          string `json:"source"`
	URL             string `json:"url"`
	FetchedAt       string `json:"fetched_at"`
	FunctionCount   int    `json:"function_count"`
	GlossaryCount   int    `json:"glossary_count"`
	IngredientCount int    `json:"ingredient_count"`
	GlossaryVersion string `json:"glossary_version"`
	License         string `json:"license"`
}

type Manifest struct {
	SchemaVersion int                      `json:"schema_version"`
	GeneratedBy   string                   `json:"generated_by"`
	FetchedAt     string                   `json:"fetched_at"`
	Annexes       map[string]ManifestAnnex `json:"annexes"`
	Mirror        ManifestMirror           `json:"mirror"`
}
