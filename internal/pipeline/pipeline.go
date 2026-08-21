package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/reconcile/internal/ingest"
	"github.com/reconcile/internal/match"
	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

type Options struct {
	ExactOnly bool
}

type PipelineResult struct {
	Matches             []model.MatchResult
	Exceptions          []model.Exception
	TotalRecordsRead    int
	RecordsBySource     map[string]int
	ExceptionsBySource  map[string]int
}

type sourceResult struct {
	sourceName string
	records    []model.Record
	exceptions []model.Exception
	err        error
}

// Run executes the full multi-source reconciliation pipeline concurrently.
func Run(ctx context.Context, cfg *rules.Config, opts Options) (*PipelineResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var expectedSources []string
	for _, s := range cfg.Sources {
		expectedSources = append(expectedSources, s.Name)
	}

	resultChan := make(chan sourceResult, len(cfg.Sources))
	var wg sync.WaitGroup

	// Phase 1: Ingest all sources concurrently
	for _, src := range cfg.Sources {
		wg.Add(1)
		go func(s rules.SourceConfig) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				resultChan <- sourceResult{sourceName: s.Name, err: ctx.Err()}
				return
			default:
			}

			records, exceptions, err := ingest.ParseCSV(s.Name, s.File)
			resultChan <- sourceResult{
				sourceName: s.Name,
				records:    records,
				exceptions: exceptions,
				err:        err,
			}
		}(src)
	}

	wg.Wait()
	close(resultChan)

	var allRecords []model.Record
	var allExceptions []model.Exception
	recordsBySource := make(map[string]int)
	exceptionsBySource := make(map[string]int)
	totalRecordsRead := 0

	for res := range resultChan {
		if res.err != nil {
			return nil, fmt.Errorf("ingest error in source %s: %w", res.sourceName, res.err)
		}
		allRecords = append(allRecords, res.records...)
		allExceptions = append(allExceptions, res.exceptions...)
		recordsBySource[res.sourceName] = len(res.records)
		exceptionsBySource[res.sourceName] = len(res.exceptions)
		totalRecordsRead += len(res.records) + len(res.exceptions)
	}

	// Phase 2: Exact Match Pass
	exactMatches, unmatchedAfterExact, duplicateExceptions := match.ExactMatchPass(
		allRecords,
		expectedSources,
		cfg.Matching.Exact,
	)
	allExceptions = append(allExceptions, duplicateExceptions...)

	var finalMatches []model.MatchResult = exactMatches
	var unmatchedAfterFuzzy []model.Record = unmatchedAfterExact

	// Phase 3: Fuzzy Match Pass (if not exact-only)
	if !opts.ExactOnly {
		fuzzyMatches, leftovers := match.FuzzyMatchPass(
			unmatchedAfterExact,
			expectedSources,
			cfg.Matching.Fuzzy,
		)
		finalMatches = append(finalMatches, fuzzyMatches...)
		unmatchedAfterFuzzy = leftovers
	}

	// Phase 4: Classify remaining unmatched records into typed exceptions
	classifiedExceptions := match.ClassifyExceptions(
		unmatchedAfterFuzzy,
		allRecords,
		expectedSources,
		cfg.Matching,
	)
	allExceptions = append(allExceptions, classifiedExceptions...)

	return &PipelineResult{
		Matches:            finalMatches,
		Exceptions:         allExceptions,
		TotalRecordsRead:   totalRecordsRead,
		RecordsBySource:    recordsBySource,
		ExceptionsBySource: exceptionsBySource,
	}, nil
}
