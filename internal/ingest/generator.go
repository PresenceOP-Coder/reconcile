package ingest

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// GenerateFixtures generates synthetic CSV data for testing with deterministic representation of all edge cases.
func GenerateFixtures(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	gatewayFile, err := os.Create(filepath.Join(outputDir, "gateway_settlement.csv"))
	if err != nil {
		return err
	}
	defer gatewayFile.Close()

	bankFile, err := os.Create(filepath.Join(outputDir, "bank_statement.csv"))
	if err != nil {
		return err
	}
	defer bankFile.Close()

	ledgerFile, err := os.Create(filepath.Join(outputDir, "internal_ledger.csv"))
	if err != nil {
		return err
	}
	defer ledgerFile.Close()

	gwWriter := csv.NewWriter(gatewayFile)
	bankWriter := csv.NewWriter(bankFile)
	ledgerWriter := csv.NewWriter(ledgerFile)

	defer gwWriter.Flush()
	defer bankWriter.Flush()
	defer ledgerWriter.Flush()

	headers := []string{"ID", "RefID", "Amount", "Currency", "Date", "Description"}
	gwWriter.Write(headers)
	bankWriter.Write(headers)
	ledgerWriter.Write(headers)

	baseDate := time.Date(2023, 10, 1, 10, 0, 0, 0, time.UTC)
	randGen := rand.New(rand.NewSource(42))

	for i := 1; i <= 25; i++ {
		refID := fmt.Sprintf("REF-%04d", i)
		amount := float64(randGen.Intn(8000)+2000) / 100.0
		currentDate := baseDate.AddDate(0, 0, i)
		dateStr := currentDate.Format(time.RFC3339)

		switch {
		case i <= 13: // 1:1 clean match
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Gateway Settlement"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})

		case i <= 15: // Split settlement (1 ledger entry = 2 gateway entries)
			amt1 := float64(int(amount*0.6*100)) / 100.0
			amt2 := float64(int((amount-amt1)*100)) / 100.0
			totalAmt := amt1 + amt2
			gwWriter.Write([]string{fmt.Sprintf("GW-%d-A", i), refID, fmt.Sprintf("%.2f", amt1), "INR", dateStr, "Split Settlement Part 1"})
			gwWriter.Write([]string{fmt.Sprintf("GW-%d-B", i), refID, fmt.Sprintf("%.2f", amt2), "INR", dateStr, "Split Settlement Part 2"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", totalAmt), "INR", dateStr, "Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", totalAmt), "INR", dateStr, "General Ledger Sale"})

		case i <= 17: // Currency conversion / rounding drift
			gwAmt := amount + 0.05
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", gwAmt), "INR", dateStr, "Gateway Settlement"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", gwAmt), "INR", dateStr, "Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})

		case i <= 19: // Settlement date lag (T+2)
			driftDateStr := currentDate.AddDate(0, 0, 2).Format(time.RFC3339)
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", driftDateStr, "Delayed Gateway Settlement"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", driftDateStr, "Delayed Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})

		case i == 20: // Amount mismatch outside tolerance
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount*1.25), "INR", dateStr, "Overcharged Gateway Entry"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})

		case i == 21: // Date drift outside window
			excessDateStr := currentDate.AddDate(0, 0, 7).Format(time.RFC3339)
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", excessDateStr, "Stale Settlement"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", excessDateStr, "Stale Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})

		case i == 22: // Duplicate RefID in source
			gwWriter.Write([]string{fmt.Sprintf("GW-%d-1", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Duplicate Gateway Batch 1"})
			gwWriter.Write([]string{fmt.Sprintf("GW-%d-2", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Duplicate Gateway Batch 2"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})

		case i == 23: // Orphan in ledger
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Unsettled Ledger Sale"})

		case i == 24: // Orphan in gateway
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Unrecorded Gateway Inflow"})

		case i == 25: // Bad date string
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", "INVALID_DATE_FORMAT_2023-99-99", "Corrupted Date Entry"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Bank Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "General Ledger Sale"})
		}
	}

	return nil
}
