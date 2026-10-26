package inci

import "testing"

const sampleAnnex = `"File creation date: 26/10/2026"
"ANNEX II","Last update: 13/10/2026"
"LIST OF SUBSTANCES PROHIBITED IN COSMETIC PRODUCTS"
"Substance identification"
"Reference Number","Chemical name / INN","CAS Number","EC Number",Regulation,"Other Directives/Regulations","SCCS opinions","Chemical/IUPAC Name","Identified INGREDIENTS or substances e.g.",CMR,"Update Date"
1388,"Octamethylcyclotetrasiloxane; D4","556-67-2","209-136-7","(EU) 2019/831","","Opinion on D4","Octamethylcyclotetrasiloxane; D4","CYCLOMETHICONE","Reprotoxic Cat. 2()","17/10/2010"
`

func TestReadAnnex(t *testing.T) {
	entries, err := ReadAnnex("II", []byte(sampleAnnex))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries", len(entries))
	}
	entry := entries[0]
	if entry.Reference != "1388" || entry.ChemicalName == "" {
		t.Fatalf("entry = %#v", entry)
	}
	found := false
	for _, name := range entry.NormalisedNames {
		if name.Name == "CYCLOMETHICONE" && name.Source == "identified" {
			found = true
		}
	}
	if !found {
		t.Fatalf("identified name missing: %#v", entry.NormalisedNames)
	}
	if entry.Qualifier != "" || entry.NanoOnly {
		t.Fatalf("unexpected qualifier/nano: %#v", entry)
	}
}

func TestSplitIdentifiedLocants(t *testing.T) {
	got := splitIdentified("N,N-DIETHYL-m-AMINOPHENOL,CI 77480,GOLD")
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	if got[0] != "N,N-DIETHYL-m-AMINOPHENOL" || got[1] != "CI 77480" || got[2] != "GOLD" {
		t.Fatalf("got %#v", got)
	}
}

func TestReadHeaderDates(t *testing.T) {
	creation, update := ReadHeaderDates([]byte(sampleAnnex))
	if creation != "26/10/2026" || update != "13/10/2026" {
		t.Fatalf("creation=%q update=%q", creation, update)
	}
}
