package ingest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GenerateLargeFixtures creates a large synthetic dataset for benchmarking.
// It writes numRecords transaction groups across the three source files,
// distributing them across the same edge case categories as the small fixtures.
func GenerateLargeFixtures(outputDir string, numRecords int) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	gwFile, err := os.Create(filepath.Join(outputDir, "gateway_settlement.csv"))
	if err != nil {
		return err
	}
	defer gwFile.Close()

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

	gw := csv.NewWriter(gwFile)
	bank := csv.NewWriter(bankFile)
	ledger := csv.NewWriter(ledgerFile)

	defer gw.Flush()
	defer bank.Flush()
	defer ledger.Flush()

	headers := []string{"ID", "RefID", "Amount", "Currency", "Date", "Description"}
	gw.Write(headers)
	bank.Write(headers)
	ledger.Write(headers)

	baseDate := time.Date(2023, 1, 1, 9, 0, 0, 0, time.UTC)

	for i := 1; i <= numRecords; i++ {
		refID := fmt.Sprintf("REF-%08d", i)
		amount := float64(1000+(i%9000)) + float64(i%100)/100.0
		date := baseDate.Add(time.Duration(i) * 15 * time.Minute)
		dateStr := date.Format(time.RFC3339)

		// Distribute across categories by modulo
		// ~65% exact, ~10% split, ~8% rounding, ~8% date drift, ~5% amount mismatch, ~4% orphan
		bucket := i % 100

		switch {
		case bucket < 65: // exact match
			gw.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Settlement"})
			bank.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Credit"})
			ledger.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		case bucket < 75: // split settlement
			amt1 := float64(int(amount*0.6*100)) / 100.0
			amt2 := float64(int((amount-amt1)*100)) / 100.0
			total := amt1 + amt2
			gw.Write([]string{fmt.Sprintf("GW-%d-A", i), refID, fmt.Sprintf("%.2f", amt1), "INR", dateStr, "Split Part 1"})
			gw.Write([]string{fmt.Sprintf("GW-%d-B", i), refID, fmt.Sprintf("%.2f", amt2), "INR", dateStr, "Split Part 2"})
			bank.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", total), "INR", dateStr, "Credit"})
			ledger.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", total), "INR", dateStr, "Sale"})

		case bucket < 83: // rounding drift
			gwAmt := amount + 0.03
			gw.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", gwAmt), "INR", dateStr, "Settlement"})
			bank.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", gwAmt), "INR", dateStr, "Credit"})
			ledger.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		case bucket < 91: // date drift (T+2)
			driftDate := date.AddDate(0, 0, 2).Format(time.RFC3339)
			gw.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", driftDate, "Delayed Settlement"})
			bank.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", driftDate, "Delayed Credit"})
			ledger.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		case bucket < 96: // amount mismatch (outside tolerance)
			gw.Write([]string{fmt.Sprintf("GW-%d", i), refID, fmt.Sprintf("%.2f", amount*1.30), "INR", dateStr, "Overcharged"})
			bank.Write([]string{fmt.Sprintf("BK-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Credit"})
			ledger.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Sale"})

		default: // orphan (ledger only)
			ledger.Write([]string{fmt.Sprintf("LD-%d", i), refID, fmt.Sprintf("%.2f", amount), "INR", dateStr, "Unsettled Sale"})
		}
	}

	return nil
}
