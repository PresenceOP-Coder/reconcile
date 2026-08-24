package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/reconcile/internal/pipeline"
)

type Summary struct {
	TotalRecords        int
	MatchedRecords      int
	ExactMatchedRecords int
	FuzzyMatchedRecords int
	MatchRatePct        float64

	LowConfidenceMatches int // False-positive risk estimate (confidence < 0.85)
	LowConfidenceRatePct float64

	ExceptionRecords    int
	ExceptionRatePct    float64
	ExceptionBreakdown  map[string]int
	ExceptionsBySource  map[string]int
	RecordsBySource     map[string]int
	CountInvariantValid bool
}

// Compute processes a PipelineResult and generates reconciliation metrics.
func Compute(res *pipeline.PipelineResult) Summary {
	s := Summary{
		TotalRecords:       res.TotalRecordsRead,
		ExceptionBreakdown: make(map[string]int),
		ExceptionsBySource: make(map[string]int),
		RecordsBySource:    res.RecordsBySource,
	}

	for _, m := range res.Matches {
		recCount := len(m.Records)
		s.MatchedRecords += recCount
		if m.Pass == "exact" {
			s.ExactMatchedRecords += recCount
		} else {
			s.FuzzyMatchedRecords += recCount
			if m.Confidence < 0.85 {
				s.LowConfidenceMatches += recCount
			}
		}
	}

	s.ExceptionRecords = len(res.Exceptions)
	for _, exc := range res.Exceptions {
		s.ExceptionBreakdown[exc.ReasonCode]++
		s.ExceptionsBySource[exc.Record.Source]++
	}

	if s.TotalRecords > 0 {
		s.MatchRatePct = (float64(s.MatchedRecords) / float64(s.TotalRecords)) * 100.0
		s.LowConfidenceRatePct = (float64(s.LowConfidenceMatches) / float64(s.TotalRecords)) * 100.0
		s.ExceptionRatePct = (float64(s.ExceptionRecords) / float64(s.TotalRecords)) * 100.0
	}

	// Verification invariant: total records must equal sum of matched and exceptions
	s.CountInvariantValid = (s.MatchedRecords + s.ExceptionRecords) == s.TotalRecords

	return s
}

// PrintSummary renders a clean summary table to the given writer.
func PrintSummary(w io.Writer, s Summary) {
	divider := strings.Repeat("=", 68)
	subDivider := strings.Repeat("-", 68)

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, divider)
	fmt.Fprintln(w, "           MULTI-SOURCE FINANCIAL RECONCILIATION REPORT           ")
	fmt.Fprintln(w, divider)

	fmt.Fprintf(w, "  Total Ingested Records    : %-6d\n", s.TotalRecords)
	fmt.Fprintf(w, "  Successfully Matched      : %-6d (%.2f%%)\n", s.MatchedRecords, s.MatchRatePct)
	fmt.Fprintf(w, "    ├── Exact Matches       : %-6d\n", s.ExactMatchedRecords)
	fmt.Fprintf(w, "    └── Fuzzy Matches       : %-6d\n", s.FuzzyMatchedRecords)
	fmt.Fprintf(w, "  Uncertainty / FP Risk     : %-6d (%.2f%% of total)\n", s.LowConfidenceMatches, s.LowConfidenceRatePct)
	fmt.Fprintf(w, "  Total Exceptions          : %-6d (%.2f%%)\n", s.ExceptionRecords, s.ExceptionRatePct)
	fmt.Fprintln(w, subDivider)

	fmt.Fprintln(w, "  EXCEPTION BREAKDOWN BY REASON CODE:")
	// Sort keys for deterministic output
	var reasonCodes []string
	for code := range s.ExceptionBreakdown {
		reasonCodes = append(reasonCodes, code)
	}
	sort.Strings(reasonCodes)

	for _, code := range reasonCodes {
		count := s.ExceptionBreakdown[code]
		pct := 0.0
		if s.ExceptionRecords > 0 {
			pct = (float64(count) / float64(s.ExceptionRecords)) * 100.0
		}
		fmt.Fprintf(w, "    • %-22s : %4d records  (%5.1f%% of exceptions)\n", code, count, pct)
	}

	fmt.Fprintln(w, subDivider)
	if s.CountInvariantValid {
		fmt.Fprintf(w, "  ✓ Integrity Invariant: %d matched + %d exceptions = %d total (VERIFIED)\n",
			s.MatchedRecords, s.ExceptionRecords, s.TotalRecords)
	} else {
		fmt.Fprintf(w, "  ✗ Integrity Invariant VIOLATED: %d matched + %d exceptions != %d total\n",
			s.MatchedRecords, s.ExceptionRecords, s.TotalRecords)
	}
	fmt.Fprintln(w, divider)
	fmt.Fprintln(w, "")
}
