package audit

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/reconcile/internal/pipeline"
)

type AuditEntry struct {
	RecordID    string   `json:"record_id"`
	Source      string   `json:"source"`
	Outcome     string   `json:"outcome"`               // "matched" | "exception"
	Pass        string   `json:"pass,omitempty"`        // "exact" | "fuzzy"
	Rule        string   `json:"rule,omitempty"`
	MatchedWith []string `json:"matched_with,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	ReasonCode  string   `json:"reason_code,omitempty"`
	Detail      string   `json:"detail,omitempty"`
}

// WriteAuditLog generates a line-delimited JSON audit file for all records.
func WriteAuditLog(outputPath string, result *pipeline.PipelineResult) (int, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create audit log file %s: %w", outputPath, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	linesWritten := 0

	// 1. Write matched records
	for _, match := range result.Matches {
		for _, rec := range match.Records {
			var counterparts []string
			for _, other := range match.Records {
				if other.ID != rec.ID {
					counterparts = append(counterparts, fmt.Sprintf("%s:%s", other.Source, other.ID))
				}
			}

			entry := AuditEntry{
				RecordID:    rec.ID,
				Source:      rec.Source,
				Outcome:     "matched",
				Pass:        match.Pass,
				Rule:        match.RuleApplied,
				MatchedWith: counterparts,
				Confidence:  match.Confidence,
			}

			if err := encoder.Encode(entry); err != nil {
				return linesWritten, fmt.Errorf("failed to encode audit log line: %w", err)
			}
			linesWritten++
		}
	}

	// 2. Write exception records
	for _, exc := range result.Exceptions {
		entry := AuditEntry{
			RecordID:   exc.Record.ID,
			Source:     exc.Record.Source,
			Outcome:    "exception",
			ReasonCode: exc.ReasonCode,
			Detail:     exc.Detail,
		}

		if err := encoder.Encode(entry); err != nil {
			return linesWritten, fmt.Errorf("failed to encode audit log line: %w", err)
		}
		linesWritten++
	}

	return linesWritten, nil
}
