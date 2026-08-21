package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reconcile/internal/rules"
)

func TestLoadValidRules(t *testing.T) {
	yamlContent := `
matching:
  exact:
    date_bucket_days: 0
  fuzzy:
    amount_tolerance_pct: 1.5
    date_window_days: 3
    min_confidence: 0.75
sources:
  - name: gateway
    file: testdata/gateway_settlement.csv
  - name: bank
    file: testdata/bank_statement.csv
  - name: ledger
    file: testdata/internal_ledger.csv
`
	tmpDir := t.TempDir()
	rulesFile := filepath.Join(tmpDir, "rules.yaml")
	if err := os.WriteFile(rulesFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp rules file: %v", err)
	}

	cfg, err := rules.Load(rulesFile)
	if err != nil {
		t.Fatalf("expected valid load, got error: %v", err)
	}

	if len(cfg.Sources) != 3 {
		t.Errorf("expected 3 sources, got %d", len(cfg.Sources))
	}
	if cfg.Matching.Fuzzy.AmountTolerancePct != 1.5 {
		t.Errorf("expected amount tolerance 1.5, got %f", cfg.Matching.Fuzzy.AmountTolerancePct)
	}
}

func TestLoadInvalidRules(t *testing.T) {
	yamlContent := `
matching:
  fuzzy:
    min_confidence: 1.5 # Invalid: must be <= 1.0
sources:
  - name: gateway
    file: test.csv
`
	tmpDir := t.TempDir()
	rulesFile := filepath.Join(tmpDir, "rules.yaml")
	os.WriteFile(rulesFile, []byte(yamlContent), 0644)

	_, err := rules.Load(rulesFile)
	if err == nil {
		t.Errorf("expected error for invalid confidence threshold, got nil")
	}
}
