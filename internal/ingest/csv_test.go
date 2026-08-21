package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reconcile/internal/ingest"
)

func TestParseCSV_ValidAndMalformed(t *testing.T) {
	csvContent := `ID,RefID,Amount,Currency,Date,Description
GW-1,REF-001,50.00,INR,2023-10-01T10:00:00Z,Valid Row
GW-2,REF-002,BAD_AMOUNT,INR,2023-10-01T10:00:00Z,Bad Amount
GW-3,REF-003,75.00,INR,NOT_A_DATE,Bad Date
GW-4,REF-004,100.00,INR,2023-10-01T10:00:00Z,Valid Row 2
`
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("failed to write temp csv: %v", err)
	}

	records, exceptions, err := ingest.ParseCSV("gateway", csvPath)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 valid records, got %d", len(records))
	}
	if len(exceptions) != 2 {
		t.Errorf("expected 2 malformed exceptions, got %d", len(exceptions))
	}

	for _, exc := range exceptions {
		if exc.ReasonCode != "MALFORMED_INPUT" {
			t.Errorf("expected ReasonCode MALFORMED_INPUT, got %s", exc.ReasonCode)
		}
	}
}
