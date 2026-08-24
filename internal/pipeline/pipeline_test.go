package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reconcile/internal/ingest"
	"github.com/reconcile/internal/pipeline"
	"github.com/reconcile/internal/rules"
)

func writeTempRules(t *testing.T, dir string) string {
	t.Helper()
	content := `
matching:
  exact:
    date_bucket_days: 0
  fuzzy:
    amount_tolerance_pct: 1.5
    date_window_days: 3
    min_confidence: 0.75
sources:
  - name: gateway
    file: ` + filepath.ToSlash(filepath.Join(dir, "gateway_settlement.csv")) + `
  - name: bank
    file: ` + filepath.ToSlash(filepath.Join(dir, "bank_statement.csv")) + `
  - name: ledger
    file: ` + filepath.ToSlash(filepath.Join(dir, "internal_ledger.csv")) + `
`
	path := filepath.Join(dir, "rules.yaml")
	os.WriteFile(path, []byte(content), 0644)
	return path
}

func TestPipelineRun_ExactOnly(t *testing.T) {
	dir := t.TempDir()
	if err := ingest.GenerateFixtures(dir); err != nil {
		t.Fatalf("fixture generation failed: %v", err)
	}

	rulesPath := writeTempRules(t, dir)
	cfg, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("rules load failed: %v", err)
	}

	result, err := pipeline.Run(context.Background(), cfg, pipeline.Options{ExactOnly: true})
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// Exact-only: some records must match with confidence 1.0
	if len(result.Matches) == 0 {
		t.Error("expected at least some exact matches")
	}
	for _, m := range result.Matches {
		if m.Pass != "exact" {
			t.Errorf("exact-only mode produced a non-exact match: pass=%s", m.Pass)
		}
		if m.Confidence != 1.0 {
			t.Errorf("exact match confidence should be 1.0, got %f", m.Confidence)
		}
	}
}

func TestPipelineRun_FullPipeline_IntegrityInvariant(t *testing.T) {
	dir := t.TempDir()
	if err := ingest.GenerateFixtures(dir); err != nil {
		t.Fatalf("fixture generation failed: %v", err)
	}

	rulesPath := writeTempRules(t, dir)
	cfg, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("rules load failed: %v", err)
	}

	result, err := pipeline.Run(context.Background(), cfg, pipeline.Options{})
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// Core invariant: no record is silently dropped
	totalMatched := 0
	for _, m := range result.Matches {
		totalMatched += len(m.Records)
	}
	totalExceptions := len(result.Exceptions)
	total := totalMatched + totalExceptions

	if total != result.TotalRecordsRead {
		t.Errorf("integrity invariant violated: matched(%d) + exceptions(%d) = %d, but total read = %d",
			totalMatched, totalExceptions, total, result.TotalRecordsRead)
	}
}

func TestPipelineRun_FuzzyMatchesDriftCases(t *testing.T) {
	dir := t.TempDir()
	if err := ingest.GenerateFixtures(dir); err != nil {
		t.Fatalf("fixture generation failed: %v", err)
	}

	rulesPath := writeTempRules(t, dir)
	cfg, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("rules load failed: %v", err)
	}

	result, err := pipeline.Run(context.Background(), cfg, pipeline.Options{})
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// The fixtures include rounding drift and date drift cases that should be
	// recovered by the fuzzy pass — make sure at least some fuzzy matches exist.
	fuzzyCount := 0
	for _, m := range result.Matches {
		if m.Pass == "fuzzy" {
			fuzzyCount++
		}
	}
	if fuzzyCount == 0 {
		t.Error("expected at least some fuzzy matches for drift cases in fixtures")
	}
}

func TestPipelineRun_MalformedInputIsolated(t *testing.T) {
	dir := t.TempDir()
	if err := ingest.GenerateFixtures(dir); err != nil {
		t.Fatalf("fixture generation failed: %v", err)
	}

	rulesPath := writeTempRules(t, dir)
	cfg, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("rules load failed: %v", err)
	}

	result, err := pipeline.Run(context.Background(), cfg, pipeline.Options{})
	if err != nil {
		t.Fatalf("pipeline should not crash on malformed input: %v", err)
	}

	malformedCount := 0
	for _, exc := range result.Exceptions {
		if exc.ReasonCode == "MALFORMED_INPUT" {
			malformedCount++
		}
	}
	if malformedCount == 0 {
		t.Error("expected at least one MALFORMED_INPUT exception from fixture's bad date row")
	}
}

func TestPipelineRun_AllExceptionTypesPresent(t *testing.T) {
	dir := t.TempDir()
	if err := ingest.GenerateFixtures(dir); err != nil {
		t.Fatalf("fixture generation failed: %v", err)
	}

	rulesPath := writeTempRules(t, dir)
	cfg, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("rules load failed: %v", err)
	}

	result, err := pipeline.Run(context.Background(), cfg, pipeline.Options{})
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	seen := make(map[string]bool)
	for _, exc := range result.Exceptions {
		seen[exc.ReasonCode] = true
	}

	// The fixtures are designed to produce every exception type
	required := []string{"AMOUNT_MISMATCH", "DATE_DRIFT", "DUPLICATE_REF", "NO_COUNTERPART", "MALFORMED_INPUT"}
	for _, code := range required {
		if !seen[code] {
			t.Errorf("expected exception code %s but it was not present in results", code)
		}
	}
}
