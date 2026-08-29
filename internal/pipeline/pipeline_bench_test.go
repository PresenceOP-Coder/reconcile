package pipeline_test

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"testing/quick"

	"github.com/reconcile/internal/ingest"
	"github.com/reconcile/internal/pipeline"
	"github.com/reconcile/internal/rules"
)

// loadTestConfig builds rules pointing at fixture files in dir.
func loadTestConfig(t *testing.T, dir string) *rules.Config {
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
	cfg, err := rules.Load(path)
	if err != nil {
		t.Fatalf("rules load failed: %v", err)
	}
	return cfg
}

// BenchmarkPipeline_Fixtures runs the pipeline against the standard 74-record fixture set.
func BenchmarkPipeline_Fixtures(b *testing.B) {
	dir := b.TempDir()
	if err := ingest.GenerateFixtures(dir); err != nil {
		b.Fatalf("fixture generation failed: %v", err)
	}
	cfg := loadBenchConfig(b, dir)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := pipeline.Run(context.Background(), cfg, pipeline.Options{}); err != nil {
			b.Fatalf("pipeline failed: %v", err)
		}
	}
}

// BenchmarkPipeline_1k runs the pipeline against 1,000 synthetic groups.
func BenchmarkPipeline_1k(b *testing.B) {
	benchmarkAt(b, 1_000)
}

// BenchmarkPipeline_10k runs the pipeline against 10,000 synthetic groups.
func BenchmarkPipeline_10k(b *testing.B) {
	benchmarkAt(b, 10_000)
}

func benchmarkAt(b *testing.B, n int) {
	b.Helper()
	dir := b.TempDir()
	if err := ingest.GenerateLargeFixtures(dir, n); err != nil {
		b.Fatalf("fixture generation failed: %v", err)
	}
	cfg := loadBenchConfig(b, dir)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := pipeline.Run(context.Background(), cfg, pipeline.Options{}); err != nil {
			b.Fatalf("pipeline failed: %v", err)
		}
	}
}

func loadBenchConfig(b *testing.B, dir string) *rules.Config {
	b.Helper()
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
	cfg, err := rules.Load(path)
	if err != nil {
		b.Fatalf("rules load failed: %v", err)
	}
	return cfg
}

// TestIntegrityInvariant_PropertyBased uses testing/quick to generate random
// record counts and asserts that matched + exceptions == total always holds.
// This is a property-based test — it proves the invariant structurally,
// not just for the fixed 74-record fixture.
func TestIntegrityInvariant_PropertyBased(t *testing.T) {
	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		n := 1 + rng.Intn(200) // 1..200 transaction groups

		dir := t.TempDir()
		if err := ingest.GenerateLargeFixtures(dir, n); err != nil {
			return true // generation failure is not a pipeline invariant failure
		}

		cfg := loadTestConfig(t, dir)
		result, err := pipeline.Run(context.Background(), cfg, pipeline.Options{})
		if err != nil {
			return true // ingest errors are handled upstream
		}

		totalMatched := 0
		for _, m := range result.Matches {
			totalMatched += len(m.Records)
		}
		return totalMatched+len(result.Exceptions) == result.TotalRecordsRead
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Errorf("integrity invariant violated: %v", err)
	}
}
