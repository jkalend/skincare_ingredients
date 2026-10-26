package inci

import (
	"html"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var (
	foldCaser = cases.Fold()

	specialSpaces = strings.NewReplacer(
		"\u00a0", " ", "\u2000", " ", "\u2001", " ", "\u2002", " ",
		"\u2003", " ", "\u2004", " ", "\u2005", " ", "\u2006", " ",
		"\u2007", " ", "\u2008", " ", "\u2009", " ", "\u200a", " ",
		"\u202f", " ", "\u205f", " ", "\u3000", " ", "\u200b", "",
	)
	dashes = strings.NewReplacer(
		"\u2010", "-", "\u2011", "-", "\u2012", "-", "\u2013", "-",
		"\u2014", "-", "\u2015", "-", "\u2212", "-", "\u00ad", "-",
	)
	quotes = strings.NewReplacer(
		"\u2019", "'", "\u2018", "'", "\u201c", "\"", "\u201d", "\"",
	)
	british = strings.NewReplacer(
		"sulphate", "sulfate", "sulphite", "sulfite", "sulphide", "sulfide",
		"sulphur", "sulfur", "colour", "color", "aluminium", "aluminum",
	)

	aroundHyphen    = regexp.MustCompile(`\s*-\s*`)
	whitespace      = regexp.MustCompile(`\s+`)
	colourIndex     = regexp.MustCompile(`(?i)^c\.?\s?[il]\.?\s*(\d{5}(?::\d{1,2})?)$`)
	colourIndexOnly = regexp.MustCompile(`^ci \d{5}(?::\d{1,2})?$`)
	percentAnn      = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*%(?:\s*(?:min|max|w/w|v/v))?\.?|\b(?:min|max|approx\.?)\s*\d+(?:[.,]\d+)?\s*%`)
	toleranceAnn    = regexp.MustCompile(`\s*(?:\+\s*/\s*-|\+/-|±)\s*(?:\d+(?:[.,]\d+)?\s*%?)?`)
)

// Fold returns the comparison form used on both sides of every match.
// It is deliberately boring: no stemming, no edit distance, no dropping words.
func Fold(name string) string {
	text := html.UnescapeString(name)
	text = norm.NFKC.String(text)
	text = specialSpaces.Replace(text)
	text = quotes.Replace(text)
	text = dashes.Replace(text)
	text = aroundHyphen.ReplaceAllString(text, "-")
	text = foldCaser.String(text)
	text = british.Replace(text)
	text = whitespace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	text = strings.Trim(text, " .,;:")
	if match := colourIndex.FindStringSubmatch(text); match != nil {
		return "ci " + match[1]
	}
	return text
}

func IsColourIndex(folded string) bool {
	return colourIndexOnly.MatchString(folded)
}

func stripAnnotations(name string) string {
	name = percentAnn.ReplaceAllString(name, " ")
	name = toleranceAnn.ReplaceAllString(name, " ")
	return name
}
