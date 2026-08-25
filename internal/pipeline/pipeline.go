package pipeline

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/reconcile/internal/ingest"
	"github.com/reconcile/internal/match"
	"github.com/reconcile/internal/model"
	"github.com/reconcile/internal/rules"
)

// Options holds configuration for the pipeline execution.
type Options struct {
	ExactOnly bool
}

// PipelineResult aggregates the final outcomes of the pipeline.
type PipelineResult struct {
	Matches            []model.MatchResult
	Exceptions         []model.Exception
	TotalRecordsRead   int
	RecordsBySource    map[string]int
	ExceptionsBySource map[string]int
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

	// Concurrent ingestion across all configured sources
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

	// 1. Exact match pass
	exactMatches, unmatchedAfterExact, duplicateExceptions := match.ExactMatchPass(
		allRecords,
		expectedSources,
		cfg.Matching.Exact,
	)
	allExceptions = append(allExceptions, duplicateExceptions...)
	finalMatches := exactMatches

	unmatchedAfterFuzzy := unmatchedAfterExact

	// 2. Fuzzy match pass (if enabled)
	if !opts.ExactOnly && cfg.Matching.Fuzzy.AmountTolerancePct > 0 {
		var fuzzyMatches []model.MatchResult
		fuzzyMatches, unmatchedAfterFuzzy = match.FuzzyMatchPass(
			unmatchedAfterExact,
			expectedSources,
			cfg.Matching.Fuzzy,
		)
		finalMatches = append(finalMatches, fuzzyMatches...)
	}

	// 3. Exception classification for remaining records
	classifiedExceptions := match.ClassifyExceptions(
		unmatchedAfterFuzzy,
		allRecords,
		expectedSources,
		cfg.Matching,
	)
	finalExceptions := append(allExceptions, classifiedExceptions...)

	// Sort matches and exceptions deterministically
	sort.Slice(finalMatches, func(i, j int) bool {
		return finalMatches[i].MatchID < finalMatches[j].MatchID
	})
	sort.Slice(finalExceptions, func(i, j int) bool {
		return finalExceptions[i].Record.ID < finalExceptions[j].Record.ID
	})

	return &PipelineResult{
		Matches:            finalMatches,
		Exceptions:         finalExceptions,
		TotalRecordsRead:   totalRecordsRead,
		RecordsBySource:    recordsBySource,
		ExceptionsBySource: exceptionsBySource,
	}, nil
}
