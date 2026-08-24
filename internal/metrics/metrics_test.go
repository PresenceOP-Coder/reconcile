package metrics_test

import (
	"testing"

	"github.com/reconcile/internal/metrics"
	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/pipeline"
)

func makeResult(exactMatches, fuzzyMatches, exceptions int, lowConfFuzzy bool) *pipeline.PipelineResult {
	var matches []model.MatchResult
	for i := 0; i < exactMatches; i++ {
		matches = append(matches, model.MatchResult{
			Pass:       "exact",
			Confidence: 1.0,
			Records:    []model.Record{{ID: "e" + string(rune('0'+i))}, {ID: "f" + string(rune('0'+i))}},
		})
	}
	for i := 0; i < fuzzyMatches; i++ {
		conf := 0.90
		if lowConfFuzzy {
			conf = 0.76 // below the 0.85 FP risk threshold
		}
		matches = append(matches, model.MatchResult{
			Pass:       "fuzzy",
			Confidence: conf,
			Records:    []model.Record{{ID: "g" + string(rune('0'+i))}, {ID: "h" + string(rune('0'+i))}},
		})
	}
	var excs []model.Exception
	for i := 0; i < exceptions; i++ {
		excs = append(excs, model.Exception{
			Record:     model.Record{ID: "x" + string(rune('0'+i)), Source: "gateway"},
			ReasonCode: "NO_COUNTERPART",
		})
	}

	total := exactMatches*2 + fuzzyMatches*2 + exceptions
	return &pipeline.PipelineResult{
		Matches:          matches,
		Exceptions:       excs,
		TotalRecordsRead: total,
		RecordsBySource:  map[string]int{"gateway": total},
	}
}

func TestCompute_MatchRate(t *testing.T) {
	result := makeResult(5, 0, 0, false)
	s := metrics.Compute(result)

	if s.MatchRatePct != 100.0 {
		t.Errorf("expected 100%% match rate, got %.2f", s.MatchRatePct)
	}
	if !s.CountInvariantValid {
		t.Error("integrity invariant should hold for a clean result")
	}
}

func TestCompute_InvariantWithExceptions(t *testing.T) {
	result := makeResult(3, 2, 4, false)
	s := metrics.Compute(result)

	if !s.CountInvariantValid {
		t.Errorf("invariant violated: %d matched + %d exceptions != %d total",
			s.MatchedRecords, s.ExceptionRecords, s.TotalRecords)
	}
}

func TestCompute_LowConfidenceFlag(t *testing.T) {
	result := makeResult(0, 3, 0, true)
	s := metrics.Compute(result)

	if s.LowConfidenceMatches == 0 {
		t.Error("expected low-confidence matches to be flagged for fuzzy matches below 0.85")
	}
	if s.LowConfidenceRatePct <= 0 {
		t.Error("expected non-zero low confidence rate")
	}
}

func TestCompute_ZeroRecords(t *testing.T) {
	result := &pipeline.PipelineResult{
		TotalRecordsRead: 0,
		RecordsBySource:  map[string]int{},
	}
	s := metrics.Compute(result)
	// Should not divide by zero
	if s.MatchRatePct != 0 {
		t.Errorf("expected 0 match rate for empty input, got %f", s.MatchRatePct)
	}
}
