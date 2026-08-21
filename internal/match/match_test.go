package match_test

import (
	"testing"
	"time"

	"github.com/reconcile/internal/match"
	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

func TestExactMatchPass(t *testing.T) {
	now := time.Now().UTC()
	records := []model.Record{
		{ID: "GW-1", Source: "gateway", RefID: "REF-1", Amount: 100.0, Date: now},
		{ID: "BK-1", Source: "bank", RefID: "REF-1", Amount: 100.0, Date: now},
		{ID: "LD-1", Source: "ledger", RefID: "REF-1", Amount: 100.0, Date: now},
		{ID: "GW-2", Source: "gateway", RefID: "REF-2", Amount: 100.0, Date: now}, // Unmatched orphan
	}

	matches, unmatched, exceptions := match.ExactMatchPass(
		records,
		[]string{"gateway", "bank", "ledger"},
		rules.ExactConfig{DateBucketDays: 0},
	)

	if len(matches) != 1 {
		t.Fatalf("expected 1 exact match, got %d", len(matches))
	}
	if len(unmatched) != 1 {
		t.Fatalf("expected 1 unmatched record, got %d", len(unmatched))
	}
	if len(exceptions) != 0 {
		t.Fatalf("expected 0 exceptions from exact pass, got %d", len(exceptions))
	}
	if matches[0].Confidence != 1.0 {
		t.Errorf("expected confidence 1.0 for exact match, got %f", matches[0].Confidence)
	}
}

func TestFuzzyMatchPass(t *testing.T) {
	now := time.Now().UTC()
	records := []model.Record{
		{ID: "GW-1", Source: "gateway", RefID: "REF-1", Amount: 100.50, Date: now.AddDate(0, 0, 1)},
		{ID: "BK-1", Source: "bank", RefID: "REF-1", Amount: 100.50, Date: now.AddDate(0, 0, 1)},
		{ID: "LD-1", Source: "ledger", RefID: "REF-1", Amount: 100.00, Date: now},
	}

	matches, unmatched := match.FuzzyMatchPass(
		records,
		[]string{"gateway", "bank", "ledger"},
		rules.FuzzyConfig{
			AmountTolerancePct: 1.5,
			DateWindowDays:     3,
			MinConfidence:      0.75,
		},
	)

	if len(matches) != 1 {
		t.Fatalf("expected 1 fuzzy match, got %d", len(matches))
	}
	if len(unmatched) != 0 {
		t.Fatalf("expected 0 unmatched records, got %d", len(unmatched))
	}
	if matches[0].Pass != "fuzzy" {
		t.Errorf("expected pass 'fuzzy', got '%s'", matches[0].Pass)
	}
}
