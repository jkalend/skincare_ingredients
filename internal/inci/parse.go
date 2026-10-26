package inci

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	preamble = `(?im)^\W{0,4}(?:` +
		`ingredients?|ingr[ée]dients?|inhaltsstoffe|zutaten|ingredienti|` +
		`ingredientes|ingredi[eë]nten|composition|sk[lł]adniki|` +
		`st[oa]f[fn]er|ainesosat|inci)` +
		`\b\s*[:\-–]?\s*`

	panelWords = `directions?|how\s+to\s+use|usage|application|warnings?|caution|` +
		`precautions?|storage|store\s+in|net\s+(?:wt|weight|vol)|` +
		`manufactured\s+(?:by|for)|marketed\s+by|distributed\s+by|made\s+in|` +
		`best\s+before|expiry|use\s+by|customer\s+care|for\s+external\s+use|` +
		`keep\s+out\s+of\s+reach|uses?|purpose|active\s+ingredients?|` +
		`inactive\s+ingredients?|other\s+information|` +
		`pr[ée]cautions?|mode\s+d.emploi|conseils?\s+d.utilisation|utilisation|` +
		`attention|fabriqu[ée]\s+(?:par|en)|distribu[ée]\s+par|contenance|` +
		`precauciones|modo\s+de\s+(?:uso|empleo|usar)|advertencias?|` +
		`conservaci[óo]n|fabricado\s+por|pa[íi]s\s+de\s+origen|` +
		`advert[êe]ncias?|conserva[çc][ãa]o|` +
		`anwendung|hinweise?|warnhinweise?|aufbewahr\w*|` +
		`hergestellt\s+(?:von|f[üu]r)|modo\s+d.uso|avvertenze|conservare|` +
		`prodotto\s+da|gebruiksaanwijzing|waarschuwing\w*|bewaren|` +
		`bruksanvisning|advarsel|oppbevar\w*`

	marker   = `(?:\+\s*/\s*-|\+/-|±)`
	mayWords = `may\s+contain|peut\s+contenir|kann\s+enthalten|puede\s+contener|` +
		`pu[òo]\s+contenere|pode\s+conter`
)

var (
	preambleRE         = regexp.MustCompile(preamble)
	terminatorRE       = regexp.MustCompile(`(?im)^\W{0,4}(?:` + panelWords + `)\b`)
	inlineTerminatorRE = regexp.MustCompile(`(?i)(?:^|[.;\n])\s*(?:` + panelWords + `)\s*:` +
		`|\b(?:if\s+swallowed|if\s+in\s+eyes|avoid\s+contact\s+with\s+` +
		`(?:the\s+)?eyes|call\s+a?\s*poison|seek\s+medical|discontinue\s+use|` +
		`keep\s+out\s+of\s+reach|for\s+external\s+use\s+only)\b` +
		`|(?:^|[.\n]|:)\s*(?:contains?|contient|cont[eé]m|contiene|contien[ei]|` +
		`inneholder|indeholder|enth[äa]lt|inneh[åa]ller|sis[äa]lt[äa][äa])\b` +
		`[^.\n]{0,90}?\d[\d.,]*\s*(?:ppm|%|mg)`)

	mayContainRE = regexp.MustCompile(`(?i)[\[(]\s*` + marker + `\s*[\])]?\s*` +
		`(?:\b(?:` + mayWords + `)\b\s*[:\-]?\s*)?` +
		`|` + marker + `\s*(?:c\.?\s?[il]\.?\s*\d{5})` +
		`|` + marker + `\s*(?:\b(?:` + mayWords + `)\b\s*[:\-]?\s*)` +
		`|\b(?:` + mayWords + `)\b\s*[:\-]?\s*`)

	blendRE         = regexp.MustCompile(`(?i)\s*\((?:and|et|und)\)\s*`)
	footnoteRE      = regexp.MustCompile(`[*†‡°^•·]+`)
	notIngredientRE = regexp.MustCompile(`^[^\p{L}\p{N}]*$`)
	parentheticalRE = regexp.MustCompile(`\(([^()]{1,80})\)`)
	synonymSlashRE  = regexp.MustCompile(`\s+/\s+`)
	polymerRE       = regexp.MustCompile(`(?i)\b(?:co|cross|homo)?polymer\b\s*$`)
	percentRE       = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*%`)
)

// Extract finds the list inside a whole label. It returns false when there is
// no credible "Ingredients / INCI / Composition" heading; guessing is worse
// than admitting that the text is not a panel.
func Extract(text string) (string, bool) {
	loc := preambleRE.FindStringIndex(text)
	if loc == nil {
		return "", false
	}
	block, _ := Trim(text[loc[1]:])
	if strings.TrimSpace(block) == "" {
		return "", false
	}
	return block, true
}

// Trim cuts a block at the first pack panel (directions, warnings, etc.) and
// removes a leading "Ingredients:" heading.
func Trim(block string) (string, string) {
	left := strings.TrimLeftFunc(block, unicode.IsSpace)
	if loc := preambleRE.FindStringIndex(left); loc != nil && loc[0] == 0 {
		left = left[loc[1]:]
	}
	end := len(left)
	if loc := terminatorRE.FindStringIndex(left); loc != nil && loc[0] < end {
		end = loc[0]
	}
	if loc := inlineTerminatorRE.FindStringIndex(left); loc != nil && loc[0] < end {
		end = loc[0]
	}
	return left[:end], left[end:]
}

// SplitTopLevel splits on separators outside brackets.
func SplitTopLevel(block string) []string {
	var out []string
	var buf strings.Builder
	depth := 0
	for _, r := range block {
		switch {
		case strings.ContainsRune("([{", r):
			depth++
			buf.WriteRune(r)
		case strings.ContainsRune(")]}", r):
			if depth > 0 {
				depth--
			}
			buf.WriteRune(r)
		case depth == 0 && (r == ',' || r == ';' || r == '\n' || r == '\r' ||
			strings.ContainsRune("•·‣|", r)):
			out = append(out, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	out = append(out, buf.String())
	return out
}

type section struct {
	text     string
	position Position
}

// SplitSections separates the declared list from a "may contain" shade block.
func SplitSections(block string) []section {
	var out []section
	cursor := 0
	for _, loc := range mayContainRE.FindAllStringIndex(block, -1) {
		if loc[0] < cursor {
			continue
		}
		if text := block[cursor:loc[0]]; strings.TrimSpace(text) != "" {
			out = append(out, section{text: text, position: Declared})
		}
		matched := block[loc[0]:loc[1]]
		opener := strings.TrimLeftFunc(matched, unicode.IsSpace)
		end := len(block)
		if len(opener) > 0 && (opener[0] == '[' || opener[0] == '(') {
			closer := byte(']')
			if opener[0] == '(' {
				closer = ')'
			}
			if found := strings.IndexByte(block[loc[1]:], closer); found >= 0 {
				end = loc[1] + found
			}
		}
		if text := block[loc[1]:end]; strings.TrimSpace(text) != "" {
			out = append(out, section{text: text, position: MayContain})
		}
		if end < len(block) {
			cursor = end + 1
		} else {
			cursor = len(block)
		}
	}
	if text := block[cursor:]; strings.TrimSpace(text) != "" {
		out = append(out, section{text: text, position: Declared})
	}
	return out
}

func cleanToken(token string) string {
	token = stripAnnotations(token)
	token = footnoteRE.ReplaceAllString(token, " ")
	token = strings.Join(strings.Fields(token), " ")
	token = strings.Trim(token, " .,;:-")
	for len(token) > 0 {
		last := token[len(token)-1]
		var open byte
		switch last {
		case ')':
			open = '('
		case ']':
			open = '['
		case '}':
			open = '{'
		default:
			return token
		}
		if strings.Count(token, string(last)) <= strings.Count(token, string(open)) {
			return token
		}
		token = strings.TrimRight(token[:len(token)-1], " .,;:-")
	}
	return token
}

func aliases(raw string) []Alias {
	foldedRaw := Fold(raw)
	var forms []Alias

	stripped := parentheticalRE.ReplaceAllString(raw, " ")
	stripped = strings.Join(strings.Fields(stripped), " ")
	stripped = strings.Trim(stripped, " .,;:-")
	if stripped != "" && Fold(stripped) != foldedRaw {
		outside := len(strings.Fields(stripped))
		inside := 0
		for _, match := range parentheticalRE.FindAllStringSubmatch(raw, -1) {
			inside += len(strings.Fields(match[1]))
		}
		kind := "whole"
		if outside < inside {
			kind = "part"
		}
		forms = append(forms, Alias{Text: stripped, Kind: kind})
	}

	slashSource := stripped
	if slashSource == "" {
		slashSource = raw
	}
	if !polymerRE.MatchString(slashSource) {
		for _, piece := range synonymSlashRE.Split(raw, -1) {
			piece = strings.Trim(piece, " .,;:-")
			if piece != "" && Fold(piece) != foldedRaw {
				forms = append(forms, Alias{Text: piece, Kind: "part"})
			}
		}
	}
	for _, match := range parentheticalRE.FindAllStringSubmatch(raw, -1) {
		for _, piece := range regexp.MustCompile(`[/,]`).Split(match[1], -1) {
			piece = strings.TrimSpace(piece)
			if piece != "" && Fold(piece) != foldedRaw {
				forms = append(forms, Alias{Text: piece, Kind: "part"})
			}
		}
	}

	var seen []Alias
	taken := map[string]bool{}
	for _, form := range forms {
		key := Fold(form.Text)
		if key != "" && !taken[key] {
			taken[key] = true
			seen = append(seen, form)
		}
	}
	return seen
}

func declaredPercent(raw string) *float64 {
	match := percentRE.FindStringSubmatch(raw)
	if match == nil {
		return nil
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", "."), 64)
	if err != nil {
		return nil
	}
	return &value
}

// Parse turns an ingredient block into ordered tokens. It never silently drops
// a token: punctuation-only pieces are skipped, prose is kept and reported.
func Parse(block string) []Ingredient {
	block = html.UnescapeString(block)
	sections := SplitSections(block)
	indices := map[Position]int{Declared: 0, MayContain: 0}
	var out []Ingredient
	for _, sec := range sections {
		for _, chunk := range SplitTopLevel(sec.text) {
			for _, piece := range blendRE.Split(chunk, -1) {
				raw := cleanToken(piece)
				if raw == "" || notIngredientRE.MatchString(raw) {
					continue
				}
				folded := Fold(raw)
				if folded == "" {
					continue
				}
				indices[sec.position]++
				out = append(out, Ingredient{
					Printed:     raw,
					Normalised:  folded,
					Index:       indices[sec.position],
					Position:    sec.position,
					DeclaredPct: declaredPercent(piece),
					Aliases:     aliases(raw),
				})
			}
		}
	}
	return out
}

func LooksLikeProse(item Ingredient) bool {
	return len(strings.Fields(item.Printed)) > 8
}

func LooksBinary(text string) bool {
	if strings.IndexByte(text, 0) >= 0 {
		return true
	}
	sample := text
	if len(sample) > 4000 {
		sample = sample[:4000]
	}
	if sample == "" {
		return false
	}
	printable := 0
	total := 0
	for _, r := range sample {
		total++
		if (strconv.IsPrint(r) || r == '\t' || r == '\n' || r == '\r') && r != '\uFFFD' {
			printable++
		}
	}
	return total > 0 && float64(printable)/float64(total) < 0.85
}
