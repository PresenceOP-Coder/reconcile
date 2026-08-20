package ingest

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// GenerateFixtures generates synthetic CSV data for testing.
func GenerateFixtures(outputDir string) error {
	rand.Seed(time.Now().UnixNano())

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

	baseDate := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	recordCount := 25 // 25 groups

	for i := 1; i <= recordCount; i++ {
		refID := fmt.Sprintf("REF-%04d", i)
		amount := float64(rand.Intn(10000)+1000) / 100.0 // 10.00 to 110.00

		caseType := rand.Intn(10)
		// 0..5 (60%): Exact match
		// 6: Split settlement
		// 7: Rounding drift
		// 8: Date drift
		// 9: Orphan or Duplicate or Malformed

		switch {
		case caseType <= 5: // Exact match
			dateStr := baseDate.Format(time.RFC3339)
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Payment"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		case caseType == 6: // Split settlement
			dateStr := baseDate.Format(time.RFC3339)
			amt1 := amount * 0.6
			amt2 := amount - amt1
			gwWriter.Write([]string{fmt.Sprintf("GW-%d-A", i), refID, fmt.Sprintf("%.2f", amt1), "INR", dateStr, "Payment Part 1"})
			gwWriter.Write([]string{fmt.Sprintf("GW-%d-B", i), refID, fmt.Sprintf("%.2f", amt2), "INR", dateStr, "Payment Part 2"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		case caseType == 7: // Rounding drift
			dateStr := baseDate.Format(time.RFC3339)
			gwAmt := amount + 0.05
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", gwAmt), "INR", dateStr, "Payment"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", gwAmt), "INR", dateStr, "Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		case caseType == 8: // Date drift
			gwDateStr := baseDate.AddDate(0, 0, 2).Format(time.RFC3339) // 2 days later
			ldDateStr := baseDate.Format(time.RFC3339)
			gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", gwDateStr, "Payment"})
			bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", gwDateStr, "Credit"})
			ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", ldDateStr, "Sale"})

		case caseType == 9: // Orphans and other errors
			subType := rand.Intn(4)
			dateStr := baseDate.Format(time.RFC3339)
			if subType == 0 { // Orphan in ledger
				ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale Orphan"})
			} else if subType == 1 { // Orphan in gateway
				gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Payment Orphan"})
			} else if subType == 2 { // Duplicate REF
				gwWriter.Write([]string{fmt.Sprintf("GW-%d-1", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Payment 1"})
				gwWriter.Write([]string{fmt.Sprintf("GW-%d-2", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Payment 2"})
				bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Credit"})
				ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})
			} else if subType == 3 { // Malformed
				// We'll write a deliberately malformed row directly without the CSV writer to break parsing later.
				gwWriter.Write([]string{fmt.Sprintf("GW-%d", i), refID, "NOT_A_NUMBER", "INR", dateStr, "Bad Record"})
				bankWriter.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Credit"})
				ledgerWriter.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})
			}
		}

		baseDate = baseDate.AddDate(0, 0, 1)
	}

	// Make sure we write one malformed date row manually just in case
	gwWriter.Write([]string{"GW-MALF", "REF-MALF", "100.00", "INR", "bad-date-format", "Malformed Row"})
	
	return nil
}
