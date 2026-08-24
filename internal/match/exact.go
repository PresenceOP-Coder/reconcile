package match

import (
	"fmt"
	"math"
	"time"

	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

// ExactMatchPass runs deterministic exact matching on records across expected sources.
func ExactMatchPass(records []model.Record, expectedSources []string, cfg rules.ExactConfig) ([]model.MatchResult, []model.Record, []model.Exception) {
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

		// Exact match requires strictly one record per expected source
		if len(group) != len(expectedSources) {
			continue
		}

		presentSources := make(map[string]model.Record)
		for _, rec := range group {
			presentSources[rec.Source] = rec
		}

		allSourcesPresent := true
		for _, src := range expectedSources {
			if _, ok := presentSources[src]; !ok {
				allSourcesPresent = false
				break
			}
		}
		if !allSourcesPresent {
			continue
		}

		// Check amounts match (allowing for float representation)
		firstAmt := group[0].Amount
		amountsMatch := true
		for _, rec := range group[1:] {
			if math.Abs(rec.Amount-firstAmt) > 0.001 {
				amountsMatch = false
				break
			}
		}
		if !amountsMatch {
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

		daysDiff := maxDate.Sub(minDate).Hours() / 24.0
		if daysDiff > float64(cfg.DateBucketDays) {
			continue
		}

		matchID := fmt.Sprintf("MATCH-EXACT-%s", refID)
		matches = append(matches, model.MatchResult{
			MatchID:     matchID,
			Records:     group,
			Pass:        "exact",
			RuleApplied: fmt.Sprintf("exact_join(date_bucket_days=%d)", cfg.DateBucketDays),
			Confidence:  1.0,
		})

		for _, rec := range group {
			matchedRecordIDs[rec.ID] = true
		}
	}

	var unmatchedRecords []model.Record
	for _, rec := range records {
		if !matchedRecordIDs[rec.ID] {
			unmatchedRecords = append(unmatchedRecords, rec)
		}
	}

	return matches, unmatchedRecords, nil
}
