package inci

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"
)

const annexHeaderRow = 4

var (
	glossaryColumns = []string{
		"Name of Common Ingredients Glossary",
		"Colour index Number / Name of Common Ingredients Glossary",
	}
	identifiedColumn  = "Identified INGREDIENTS or substances e.g."
	wordingColumn     = "Wording of conditions of use and warnings"
	productTypeColumn = "Product Type, body parts"
	maxConcColumn     = "Maximum concentration in ready for use preparation"

	glossarySeparator = regexp.MustCompile(`[;/]`)
	emptyName         = map[string]bool{"": true, "-": true, "--": true, "n/a": true, "na": true}

	wordLocants = `alpha|beta|gamma|delta|epsilon|omega|cis|trans|syn|anti|endo|exo|` +
		`ortho|meta|para|sym|tert|sec|iso|neo|erythro|threo`
	locantTail     = regexp.MustCompile(`(?:\d|')$`)
	locantPiece    = regexp.MustCompile(`(?i)^(?:[a-z]'?|\d{1,2}'?|` + wordLocants + `)$`)
	locantHead     = regexp.MustCompile(`(?i)^(?:\d|[a-z]'?[-,])`)
	wordLocantHead = regexp.MustCompile(`(?i)^(?:` + wordLocants + `)[-,(]`)

	qualifierRE = regexp.MustCompile(`(?i)\b(except(?:\s+if| for| where)?|unless|when used|other than|` +
		`with the exception of|only (?:if|when)|provided that|if)\b`)
	nanoRE = regexp.MustCompile(`(?i)\(\s*nano\s*\)|\bnanomaterial\b|\bnanoparticle`)
)

func splitGlossary(value string) []string {
	var out []string
	for _, part := range glossarySeparator.Split(value, -1) {
		part = strings.TrimSpace(part)
		if !emptyName[strings.ToLower(part)] {
			out = append(out, part)
		}
	}
	return out
}

func isLocantRun(fragment string) bool {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return false
	}
	for _, piece := range strings.Split(fragment, ",") {
		if !locantPiece.MatchString(strings.TrimSpace(piece)) {
			return false
		}
	}
	return true
}

func splitIdentified(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	fragments := strings.Split(value, ",")
	merged := []string{fragments[0]}
	for _, fragment := range fragments[1:] {
		previous := strings.TrimRight(merged[len(merged)-1], " ")
		run := isLocantRun(previous)
		joins := locantHead.MatchString(fragment) ||
			(run && (wordLocantHead.MatchString(fragment) || isLocantRun(fragment)))
		if (locantTail.MatchString(previous) || run) && joins {
			merged[len(merged)-1] = merged[len(merged)-1] + "," + fragment
		} else {
			merged = append(merged, fragment)
		}
	}
	var out []string
	for _, part := range merged {
		part = strings.TrimSpace(part)
		if !emptyName[strings.ToLower(part)] {
			out = append(out, part)
		}
	}
	return out
}

func qualifierIn(name string) string {
	return qualifierRE.FindString(name)
}

func isNanoOnly(chemical string, names []string) bool {
	if !nanoRE.MatchString(chemical) {
		return false
	}
	for _, name := range names {
		if nanoRE.MatchString(name) {
			return false
		}
	}
	return true
}

func rowMap(header, row []string) map[string]string {
	out := make(map[string]string, len(header))
	for i, name := range header {
		if i < len(row) {
			out[name] = row[i]
		} else {
			out[name] = ""
		}
	}
	return out
}

// ReadAnnex parses one Commission CSV export. It is intentionally strict about
// the header: if the Commission changes the export, a silent empty register is
// worse than a loud failure.
func ReadAnnex(annex string, data []byte) ([]AnnexEntry, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("annex %s: %w", annex, err)
	}
	if len(rows) <= annexHeaderRow {
		return nil, fmt.Errorf("annex %s: file too short to hold a header", annex)
	}
	header := rows[annexHeaderRow]
	glossaryColumn := ""
	for _, candidate := range glossaryColumns {
		for _, cell := range header {
			if cell == candidate {
				glossaryColumn = candidate
				break
			}
		}
		if glossaryColumn != "" {
			break
		}
	}

	var entries []AnnexEntry
	for _, raw := range rows[annexHeaderRow+1:] {
		blank := true
		for _, cell := range raw {
			if strings.TrimSpace(cell) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		row := rowMap(header, raw)
		var names []string
		var normalised []NormalizedName
		seen := map[string]bool{}
		add := func(name, source string) {
			name = strings.TrimSpace(name)
			key := Fold(name)
			if key == "" || seen[key] {
				return
			}
			seen[key] = true
			names = append(names, name)
			normalised = append(normalised, NormalizedName{Name: name, Source: source})
		}
		if glossaryColumn != "" {
			source := "glossary"
			if strings.HasPrefix(glossaryColumn, "Colour index") {
				source = "colour-index"
			}
			for _, name := range splitGlossary(row[glossaryColumn]) {
				add(name, source)
			}
		}
		for _, name := range splitIdentified(row[identifiedColumn]) {
			add(name, "identified")
		}

		chemical := strings.TrimSpace(row["Chemical name / INN"])
		if chemical == "" {
			chemical = strings.TrimSpace(row["Chemical name"])
		}
		entries = append(entries, AnnexEntry{
			Annex:            annex,
			Reference:        strings.TrimSpace(row["Reference Number"]),
			ChemicalName:     chemical,
			Names:            names,
			NormalisedNames:  normalised,
			Wording:          strings.TrimSpace(row[wordingColumn]),
			ProductTypes:     strings.TrimSpace(row[productTypeColumn]),
			MaxConcentration: strings.TrimSpace(row[maxConcColumn]),
			UpdateDate:       strings.TrimSpace(row["Update Date"]),
			Qualifier:        qualifierIn(chemical),
			NanoOnly:         isNanoOnly(chemical, names),
		})
	}
	return entries, nil
}

// ReadHeaderDates returns (file creation date, Commission last-update date).
func ReadHeaderDates(data []byte) (string, string) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 {
		return "", ""
	}
	afterColon := func(value string) string {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(value)
	}
	creation := ""
	if len(rows[0]) > 0 {
		creation = afterColon(rows[0][0])
	}
	update := ""
	if len(rows) > 1 && len(rows[1]) > 1 {
		update = afterColon(rows[1][1])
	}
	return creation, update
}
