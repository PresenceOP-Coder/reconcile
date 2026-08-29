package match

import (
	"fmt"
	"math"
	"time"

	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

// FuzzyMatchPass performs tolerance-based matching on remaining records.
func FuzzyMatchPass(records []model.Record, expectedSources []string, cfg rules.FuzzyConfig) ([]model.MatchResult, []model.Record) {
	// Group records by RefID
	refGroups := make(map[string][]model.Record)
	for _, rec := range records {
		refGroups[rec.RefID] = append(refGroups[rec.RefID], rec)
	}

	var matches []model.MatchResult
	matchedRecordIDs := make(map[string]bool)

	for refID, group := range refGroups {
		if refID == "" {
			continue
		}

		// Check which sources are present in this refID group
		bySource := make(map[string][]model.Record)
		for _, rec := range group {
			bySource[rec.Source] = append(bySource[rec.Source], rec)
		}

		// All expected sources must be represented (either 1 record or multiple split records)
		allSourcesPresent := true
		for _, src := range expectedSources {
			if len(bySource[src]) == 0 {
				allSourcesPresent = false
				break
			}
		}

		if !allSourcesPresent {
			// Cannot match if a source is completely missing
			continue
		}

		// Calculate total amount per source in base currency (INR).
		// This makes the tolerance check currency-aware for multi-currency batches.
		fx := cfg.FXRates
		if fx == nil {
			fx = rules.DefaultFXRates
		}
		sourceSums := make(map[string]float64)
		for src, recs := range bySource {
			var sum float64
			for _, r := range recs {
				sum += fx.ToBase(r.Amount, r.Currency)
			}
			sourceSums[src] = sum
		}

		// Find maximum relative amount difference across any pair of sources
		var maxDiffPct float64
		var baseAmt float64
		for _, src1 := range expectedSources {
			for _, src2 := range expectedSources {
				if src1 == src2 {
					continue
				}
				s1, s2 := sourceSums[src1], sourceSums[src2]
				maxS := math.Max(s1, s2)
				if maxS > 0 {
					diffPct := (math.Abs(s1-s2) / maxS) * 100.0
					if diffPct > maxDiffPct {
						maxDiffPct = diffPct
						baseAmt = maxS
					}
				}
			}
		}

		if maxDiffPct > cfg.AmountTolerancePct {
			continue
		}

		var minDate, maxDate time.Time
		for i, rec := range group {
			if i == 0 || rec.Date.Before(minDate) {
				minDate = rec.Date
			}
			if i == 0 || rec.Date.After(maxDate) {
				maxDate = rec.Date
			}
		}

		dateDiffDays := maxDate.Sub(minDate).Hours() / 24.0
		if dateDiffDays > float64(cfg.DateWindowDays) {
			continue
		}

		// Calculate confidence based on tolerance consumption
		var amtUtil float64
		if cfg.AmountTolerancePct > 0 {
			amtUtil = maxDiffPct / cfg.AmountTolerancePct
		}
		var dateUtil float64
		if cfg.DateWindowDays > 0 {
			dateUtil = dateDiffDays / float64(cfg.DateWindowDays)
		}

		weightedUtil := (0.7 * amtUtil) + (0.3 * dateUtil)
		confidence := 1.0 - (0.30 * weightedUtil)
		confidence = math.Round(confidence*1000) / 1000.0

		if confidence < cfg.MinConfidence {
			continue
		}

		ruleDesc := fmt.Sprintf("fuzzy(amount_tol=%.2f%%, date_window=%dd, actual_diff=%.2f%%, date_diff=%.1fd)",
			cfg.AmountTolerancePct, cfg.DateWindowDays, maxDiffPct, dateDiffDays)
		if len(group) > len(expectedSources) {
			ruleDesc = fmt.Sprintf("split_settlement_%s", ruleDesc)
		}

		matchID := fmt.Sprintf("MATCH-FUZZY-%s", refID)
		matches = append(matches, model.MatchResult{
			MatchID:     matchID,
			Records:     group,
			Pass:        "fuzzy",
			RuleApplied: ruleDesc,
			Confidence:  confidence,
		})

		for _, rec := range group {
			matchedRecordIDs[rec.ID] = true
		}
		_ = baseAmt
	}

	var unmatchedRecords []model.Record
	for _, rec := range records {
		if !matchedRecordIDs[rec.ID] {
			unmatchedRecords = append(unmatchedRecords, rec)
		}
	}

	return matches, unmatchedRecords
}
