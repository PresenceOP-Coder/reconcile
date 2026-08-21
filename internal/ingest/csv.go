package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/reconcile/internal/model"
)

// ParseCSV reads a source CSV file and returns valid records and malformed record exceptions.
func ParseCSV(sourceName, filePath string) ([]model.Record, []model.Exception, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open file %s: %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Allow variable number of fields so we can catch row-length anomalies cleanly as exceptions
	reader.FieldsPerRecord = -1

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read CSV header in %s: %w", filePath, err)
	}

	headerMap := make(map[string]int)
	for i, col := range header {
		headerMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	var records []model.Record
	var exceptions []model.Exception
	rowNum := 1

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			exceptions = append(exceptions, model.Exception{
				Record: model.Record{
					ID:     fmt.Sprintf("%s-ROW-%d", sourceName, rowNum),
					Source: sourceName,
				},
				ReasonCode: "MALFORMED_INPUT",
				Detail:     fmt.Sprintf("Row %d: CSV read error: %v", rowNum, err),
			})
			continue
		}

		if len(row) < 6 {
			exceptions = append(exceptions, model.Exception{
				Record: model.Record{
					ID:     fmt.Sprintf("%s-ROW-%d", sourceName, rowNum),
					Source: sourceName,
				},
				ReasonCode: "MALFORMED_INPUT",
				Detail:     fmt.Sprintf("Row %d: expected at least 6 columns, got %d", rowNum, len(row)),
			})
			continue
		}

		id := strings.TrimSpace(row[0])
		refID := strings.TrimSpace(row[1])
		amountStr := strings.TrimSpace(row[2])
		currency := strings.TrimSpace(row[3])
		dateStr := strings.TrimSpace(row[4])
		desc := strings.TrimSpace(row[5])

		// Validate Amount
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			exceptions = append(exceptions, model.Exception{
				Record: model.Record{
					ID:          id,
					Source:      sourceName,
					RefID:       refID,
					Currency:    currency,
					Description: desc,
				},
				ReasonCode: "MALFORMED_INPUT",
				Detail:     fmt.Sprintf("Row %d (ID %s): invalid amount '%s': %v", rowNum, id, amountStr, err),
			})
			continue
		}

		// Validate Date
		parsedDate, err := parseDate(dateStr)
		if err != nil {
			exceptions = append(exceptions, model.Exception{
				Record: model.Record{
					ID:          id,
					Source:      sourceName,
					RefID:       refID,
					Amount:      amount,
					Currency:    currency,
					Description: desc,
				},
				ReasonCode: "MALFORMED_INPUT",
				Detail:     fmt.Sprintf("Row %d (ID %s): invalid date format '%s'", rowNum, id, dateStr),
			})
			continue
		}

		records = append(records, model.Record{
			ID:          id,
			Source:      sourceName,
			RefID:       refID,
			Amount:      amount,
			Currency:    currency,
			Date:        parsedDate,
			Description: desc,
		})
	}

	return records, exceptions, nil
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"02/01/2006",
		"02-01-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %s", dateStr)
}
