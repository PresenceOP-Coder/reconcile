package match

import (
	"fmt"
	"math"
	"strings"

	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

// ClassifyExceptions classifies all unmatched records into typed reason codes.
func ClassifyExceptions(unmatchedRecords []model.Record, allRecords []model.Record, expectedSources []string, cfg rules.MatchingConfig) []model.Exception {
	// Build a reference lookup from all parsed records
	allByRefID := make(map[string][]model.Record)
	for _, rec := range allRecords {
		if rec.RefID != "" {
			allByRefID[rec.RefID] = append(allByRefID[rec.RefID], rec)
		}
	}

	var exceptions []model.Exception

	for _, rec := range unmatchedRecords {
		candidates := allByRefID[rec.RefID]

		// Filter candidates from other sources
		var otherSources []model.Record
		for _, cand := range candidates {
			if cand.Source != rec.Source {
				otherSources = append(otherSources, cand)
			}
		}

		if len(otherSources) == 0 {
			exceptions = append(exceptions, model.Exception{
				Record:     rec,
				ReasonCode: "NO_COUNTERPART",
				Detail:     fmt.Sprintf("No counterpart record found with RefID '%s' in other sources", rec.RefID),
			})
			continue
		}

		// Counterparts exist - analyze why they failed to match
		bySource := make(map[string][]model.Record)
		for _, c := range candidates {
			bySource[c.Source] = append(bySource[c.Source], c)
		}

		// Check if any expected source is completely missing
		var missingSources []string
		for _, src := range expectedSources {
			if len(bySource[src]) == 0 {
				missingSources = append(missingSources, src)
			}
		}

		if len(missingSources) > 0 {
			exceptions = append(exceptions, model.Exception{
				Record:     rec,
				ReasonCode: "NO_COUNTERPART",
				Detail: fmt.Sprintf("Missing counterpart record from source(s): %s for RefID '%s'",
					strings.Join(missingSources, ", "), rec.RefID),
			})
			continue
		}

		// Check if current source or any source has duplicate records that caused ambiguity
		hasDuplicates := false
		var duplicateSource string
		var dupCount int
		for src, recs := range bySource {
			if len(recs) > 1 {
				hasDuplicates = true
				duplicateSource = src
				dupCount = len(recs)
				break
			}
		}

		// All sources exist, check amount sums
		sourceSums := make(map[string]float64)
		for src, recs := range bySource {
			var s float64
			for _, r := range recs {
				s += r.Amount
			}
			sourceSums[src] = s
		}

		var maxDiffPct float64
		for _, s1 := range expectedSources {
			for _, s2 := range expectedSources {
				if s1 == s2 {
					continue
				}
				v1, v2 := sourceSums[s1], sourceSums[s2]
				maxV := math.Max(v1, v2)
				if maxV > 0 {
					dPct := (math.Abs(v1-v2) / maxV) * 100.0
					if dPct > maxDiffPct {
						maxDiffPct = dPct
					}
				}
			}
		}

		if hasDuplicates && maxDiffPct > cfg.Fuzzy.AmountTolerancePct {
			exceptions = append(exceptions, model.Exception{
				Record:     rec,
				ReasonCode: "DUPLICATE_REF",
				Detail: fmt.Sprintf("Source '%s' has %d ambiguous records with RefID '%s' with sum variance %.2f%%",
					duplicateSource, dupCount, rec.RefID, maxDiffPct),
			})
			continue
		}

		if maxDiffPct > cfg.Fuzzy.AmountTolerancePct {
			var breakdown []string
			for src, sum := range sourceSums {
				breakdown = append(breakdown, fmt.Sprintf("%s=%.2f", src, sum))
			}
			exceptions = append(exceptions, model.Exception{
				Record:     rec,
				ReasonCode: "AMOUNT_MISMATCH",
				Detail: fmt.Sprintf("Amount delta %.2f%% exceeds tolerance %.2f%% (%s)",
					maxDiffPct, cfg.Fuzzy.AmountTolerancePct, strings.Join(breakdown, ", ")),
			})
			continue
		}

		// Check date difference
		var minDate, maxDate = candidates[0].Date, candidates[0].Date
		for _, c := range candidates[1:] {
			if c.Date.Before(minDate) {
				minDate = c.Date
			}
			if c.Date.After(maxDate) {
				maxDate = c.Date
			}
		}
		daysDiff := maxDate.Sub(minDate).Hours() / 24.0

		if daysDiff > float64(cfg.Fuzzy.DateWindowDays) {
			exceptions = append(exceptions, model.Exception{
				Record:     rec,
				ReasonCode: "DATE_DRIFT",
				Detail: fmt.Sprintf("Date difference %.1f days exceeds window %d days (min: %s, max: %s)",
					daysDiff, cfg.Fuzzy.DateWindowDays, minDate.Format("2006-01-02"), maxDate.Format("2006-01-02")),
			})
			continue
		}

		// If amount and date were borderline, confidence was below threshold
		exceptions = append(exceptions, model.Exception{
			Record:     rec,
			ReasonCode: "AMOUNT_MISMATCH",
			Detail: fmt.Sprintf("Combined match confidence did not meet required threshold of %.2f",
				cfg.Fuzzy.MinConfidence),
		})
	}

	return exceptions
}
