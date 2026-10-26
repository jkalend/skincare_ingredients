package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jkalend/skincare_ingredients/internal/testutil"
)

func TestAnalyzeEnvelope(t *testing.T) {
	_, dataDir, localDir := testutil.Setup(t)
	var stdout, stderr bytes.Buffer
	exit := Run([]string{"--data-dir", dataDir, "--local-dir", localDir, "analyze", "--text", "Aqua, Glycerin"}, nil, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["ok"] != true || envelope["exit"].(float64) != 0 {
		t.Fatalf("envelope = %#v", envelope)
	}
	data := envelope["data"].(map[string]any)
	ingredients := data["ingredients"].([]any)
	if len(ingredients) != 2 {
		t.Fatalf("ingredients = %#v", ingredients)
	}
	if envelope["warnings"] == nil || envelope["work_orders"] == nil {
		t.Fatalf("arrays must be present: %#v", envelope)
	}
}

func TestOfflineLookup(t *testing.T) {
	_, dataDir, localDir := testutil.Setup(t)
	var stdout, stderr bytes.Buffer
	exit := Run([]string{"--data-dir", dataDir, "--local-dir", localDir, "--offline", "lookup", "1000034026006"}, nil, &stdout, &stderr)
	if exit != 3 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"offline"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestParseHumanMode(t *testing.T) {
	_, dataDir, localDir := testutil.Setup(t)
	var stdout, stderr bytes.Buffer
	exit := Run([]string{"--data-dir", dataDir, "--local-dir", localDir, "--text", "analyze", "--text", "Aqua, Glycerin"}, nil, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "AQUA") {
		t.Fatalf("human output = %s", stdout.String())
	}
}
