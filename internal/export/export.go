package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/reconcile/internal/metrics"
	"github.com/reconcile/internal/pipeline"
)

// WriteExceptionsCSV dumps all exception records to a CSV file that
// finance teams can open directly in Excel.
func WriteExceptionsCSV(path string, result *pipeline.PipelineResult) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("cannot create exceptions CSV: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"RecordID", "Source", "RefID", "Amount", "Currency", "Date", "Description", "ReasonCode", "Detail"})

	for _, exc := range result.Exceptions {
		r := exc.Record
		dateStr := ""
		if !r.Date.IsZero() {
			dateStr = r.Date.Format("2006-01-02")
		}
		w.Write([]string{
			r.ID, r.Source, r.RefID,
			fmt.Sprintf("%.2f", r.Amount),
			r.Currency,
			dateStr,
			r.Description,
			exc.ReasonCode,
			exc.Detail,
		})
	}

	return len(result.Exceptions), nil
}

// WriteMatchesCSV dumps matched transaction groups to a CSV file showing
// which records were joined together and with what confidence.
func WriteMatchesCSV(path string, result *pipeline.PipelineResult) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("cannot create matches CSV: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"MatchID", "Pass", "Confidence", "Rule", "RecordIDs", "Sources", "RefID", "Amount"})

	for _, m := range result.Matches {
		var ids, sources []string
		var refID string
		var totalAmt float64
		for _, r := range m.Records {
			ids = append(ids, r.ID)
			sources = append(sources, r.Source)
			refID = r.RefID
			totalAmt += r.Amount
		}
		// Average for 1:1, sum for split settlements makes it confusing; just show first record amount
		firstAmt := m.Records[0].Amount
		w.Write([]string{
			m.MatchID,
			m.Pass,
			fmt.Sprintf("%.3f", m.Confidence),
			m.RuleApplied,
			strings.Join(ids, " | "),
			strings.Join(sources, " | "),
			refID,
			fmt.Sprintf("%.2f", firstAmt),
		})
		_ = totalAmt
	}

	return len(result.Matches), nil
}

// htmlEscape escapes a string for safe inclusion in HTML.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// WriteHTMLReport generates a self-contained HTML report with match statistics,
// exception breakdown, and an audit trail table. Useful for demos and judge reviews.
func WriteHTMLReport(path string, result *pipeline.PipelineResult, summary metrics.Summary) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create HTML report: %w", err)
	}
	defer f.Close()

	generatedAt := time.Now().Format("2006-01-02 15:04:05")

	// Sort exception reason codes for consistent output
	var codes []string
	for code := range summary.ExceptionBreakdown {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	exceptionRows := ""
	for _, code := range codes {
		count := summary.ExceptionBreakdown[code]
		pct := 0.0
		if summary.ExceptionRecords > 0 {
			pct = float64(count) / float64(summary.ExceptionRecords) * 100
		}
		exceptionRows += fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>%.1f%%</td></tr>`, htmlEscape(code), count, pct)
	}

	auditRows := ""
	for _, m := range result.Matches {
		for _, r := range m.Records {
			var counterparts []string
			for _, other := range m.Records {
				if other.ID != r.ID {
					counterparts = append(counterparts, other.Source+":"+other.ID)
				}
			}
			auditRows += fmt.Sprintf(
				`<tr class="matched"><td>%s</td><td>%s</td><td class="badge match">matched</td><td>%s</td><td>%.3f</td><td>%s</td><td></td></tr>`,
				htmlEscape(r.ID), htmlEscape(r.Source), htmlEscape(m.Pass),
				m.Confidence, htmlEscape(strings.Join(counterparts, ", ")),
			)
		}
	}
	for _, exc := range result.Exceptions {
		auditRows += fmt.Sprintf(
			`<tr class="exception"><td>%s</td><td>%s</td><td class="badge exc">exception</td><td></td><td></td><td></td><td><b>%s</b>: %s</td></tr>`,
			htmlEscape(exc.Record.ID), htmlEscape(exc.Record.Source),
			htmlEscape(exc.ReasonCode), htmlEscape(exc.Detail),
		)
	}

	invariantColor := "#d4edda"
	invariantIcon := "✓"
	invariantMsg := fmt.Sprintf("Verified: %d matched + %d exceptions = %d total",
		summary.MatchedRecords, summary.ExceptionRecords, summary.TotalRecords)
	if !summary.CountInvariantValid {
		invariantColor = "#f8d7da"
		invariantIcon = "✗"
		invariantMsg = fmt.Sprintf("VIOLATED: %d matched + %d exceptions ≠ %d total",
			summary.MatchedRecords, summary.ExceptionRecords, summary.TotalRecords)
	}

	// Marshal per-source stats for the JS chart
	sourceLabels, sourceValues := ``, ``
	for src, count := range summary.RecordsBySource {
		sourceLabels += fmt.Sprintf(`"%s",`, src)
		sourceValues += fmt.Sprintf(`%d,`, count)
	}

	excLabels, excValues := buildExcChartData(codes, summary.ExceptionBreakdown)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Reconciliation Report</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f5f7fa; color: #333; }
  header { background: #1a1a2e; color: #fff; padding: 24px 40px; }
  header h1 { font-size: 22px; font-weight: 600; }
  header p { font-size: 13px; opacity: 0.7; margin-top: 4px; }
  .container { max-width: 1200px; margin: 0 auto; padding: 32px 24px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; margin-bottom: 32px; }
  .card { background: #fff; border-radius: 10px; padding: 20px 24px; box-shadow: 0 1px 4px rgba(0,0,0,0.07); }
  .card .label { font-size: 12px; color: #888; text-transform: uppercase; letter-spacing: 0.5px; }
  .card .value { font-size: 28px; font-weight: 700; margin-top: 6px; }
  .card .sub { font-size: 12px; color: #aaa; margin-top: 2px; }
  .green { color: #2ecc71; } .orange { color: #e67e22; } .red { color: #e74c3c; } .blue { color: #3498db; }
  .charts { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin-bottom: 32px; }
  .chart-box { background: #fff; border-radius: 10px; padding: 24px; box-shadow: 0 1px 4px rgba(0,0,0,0.07); }
  .chart-box h3 { font-size: 14px; font-weight: 600; margin-bottom: 16px; color: #555; }
  table { width: 100%%; border-collapse: collapse; font-size: 13px; }
  th { background: #f0f2f5; font-weight: 600; text-align: left; padding: 10px 14px; border-bottom: 2px solid #ddd; }
  td { padding: 9px 14px; border-bottom: 1px solid #eee; vertical-align: top; }
  tr:hover td { background: #fafbfc; }
  .badge { font-size: 11px; font-weight: 600; padding: 3px 8px; border-radius: 12px; display: inline-block; }
  .badge.match { background: #d4edda; color: #155724; }
  .badge.exc { background: #f8d7da; color: #721c24; }
  .invariant { background: %s; border-radius: 8px; padding: 14px 20px; margin-bottom: 32px; font-size: 14px; font-weight: 500; }
  .section-title { font-size: 16px; font-weight: 700; margin-bottom: 16px; color: #222; }
  .table-wrap { background: #fff; border-radius: 10px; box-shadow: 0 1px 4px rgba(0,0,0,0.07); overflow: hidden; margin-bottom: 32px; overflow-x: auto; }
  @media (max-width: 768px) { .charts { grid-template-columns: 1fr; } }
</style>
</head>
<body>
<header>
  <h1>Multi-Source Financial Reconciliation Report</h1>
  <p>Generated at %s</p>
</header>
<div class="container">
  <div class="grid">
    <div class="card"><div class="label">Total Records</div><div class="value blue">%d</div><div class="sub">across all sources</div></div>
    <div class="card"><div class="label">Match Rate</div><div class="value green">%.1f%%</div><div class="sub">%d records matched</div></div>
    <div class="card"><div class="label">Exact Matches</div><div class="value green">%d</div><div class="sub">confidence 1.0</div></div>
    <div class="card"><div class="label">Fuzzy Matches</div><div class="value blue">%d</div><div class="sub">within tolerance</div></div>
    <div class="card"><div class="label">Exceptions</div><div class="value red">%d</div><div class="sub">%.1f%% exception rate</div></div>
    <div class="card"><div class="label">FP Risk (low conf)</div><div class="value orange">%d</div><div class="sub">fuzzy matches &lt;0.85</div></div>
  </div>
  <div class="invariant">%s %s</div>
  <div class="charts">
    <div class="chart-box"><h3>Match Breakdown</h3><canvas id="matchChart" height="200"></canvas></div>
    <div class="chart-box"><h3>Exception Breakdown by Reason</h3><canvas id="excChart" height="200"></canvas></div>
  </div>
  <div class="section-title">Exception Breakdown</div>
  <div class="table-wrap">
    <table><thead><tr><th>Reason Code</th><th>Count</th><th>%% of Exceptions</th></tr></thead>
    <tbody>%s</tbody></table>
  </div>
  <div class="section-title">Audit Trail</div>
  <div class="table-wrap">
    <table><thead><tr><th>Record ID</th><th>Source</th><th>Outcome</th><th>Pass</th><th>Confidence</th><th>Matched With</th><th>Exception Detail</th></tr></thead>
    <tbody>%s</tbody></table>
  </div>
</div>
<script>
new Chart(document.getElementById('matchChart'), {
  type: 'doughnut',
  data: { labels: ['Exact','Fuzzy','Exception'], datasets: [{ data: [%d,%d,%d], backgroundColor: ['#2ecc71','#3498db','#e74c3c'], borderWidth: 0 }] },
  options: { plugins: { legend: { position: 'bottom' } }, cutout: '60%%' }
});
new Chart(document.getElementById('excChart'), {
  type: 'bar',
  data: { labels: [%s], datasets: [{ label: 'Count', data: [%s], backgroundColor: '#e74c3c66', borderColor: '#e74c3c', borderWidth: 1 }] },
  options: { plugins: { legend: { display: false } }, scales: { y: { beginAtZero: true, ticks: { stepSize: 1 } } } }
});
</script>
</body>
</html>`,
		invariantColor,
		generatedAt,
		summary.TotalRecords,
		summary.MatchRatePct, summary.MatchedRecords,
		summary.ExactMatchedRecords,
		summary.FuzzyMatchedRecords,
		summary.ExceptionRecords, summary.ExceptionRatePct,
		summary.LowConfidenceMatches,
		invariantIcon, invariantMsg,
		exceptionRows,
		auditRows,
		summary.ExactMatchedRecords, summary.FuzzyMatchedRecords, summary.ExceptionRecords,
		excLabels, excValues,
	)

	_, err = f.WriteString(html)
	return err
}

func buildExcChartData(codes []string, breakdown map[string]int) (string, string) {
	labels := ""
	values := ""
	for _, c := range codes {
		labels += fmt.Sprintf(`"%s",`, c)
		values += fmt.Sprintf(`%d,`, breakdown[c])
	}
	return labels, values
}

// ExportJSON writes a structured JSON summary of the entire reconciliation run.
// Good for piping into downstream systems or dashboards.
func ExportJSON(path string, result *pipeline.PipelineResult, summary metrics.Summary) error {
	type jsonReport struct {
		GeneratedAt      string         `json:"generated_at"`
		TotalRecords     int            `json:"total_records"`
		MatchRate        float64        `json:"match_rate_pct"`
		ExactMatches     int            `json:"exact_matches"`
		FuzzyMatches     int            `json:"fuzzy_matches"`
		LowConfidence    int            `json:"low_confidence_matches"`
		ExceptionCount   int            `json:"exception_count"`
		ExceptionRate    float64        `json:"exception_rate_pct"`
		InvariantValid   bool           `json:"integrity_invariant_valid"`
		ExceptionsByCode map[string]int `json:"exceptions_by_code"`
		RecordsBySource  map[string]int `json:"records_by_source"`
	}

	report := jsonReport{
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		TotalRecords:     summary.TotalRecords,
		MatchRate:        summary.MatchRatePct,
		ExactMatches:     summary.ExactMatchedRecords,
		FuzzyMatches:     summary.FuzzyMatchedRecords,
		LowConfidence:    summary.LowConfidenceMatches,
		ExceptionCount:   summary.ExceptionRecords,
		ExceptionRate:    summary.ExceptionRatePct,
		InvariantValid:   summary.CountInvariantValid,
		ExceptionsByCode: summary.ExceptionBreakdown,
		RecordsBySource:  summary.RecordsBySource,
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create JSON report: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
