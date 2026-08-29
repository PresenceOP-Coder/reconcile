package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/reconcile/internal/agent"
	"github.com/reconcile/internal/audit"
	"github.com/reconcile/internal/export"
	"github.com/reconcile/internal/ingest"
	"github.com/reconcile/internal/metrics"
	"github.com/reconcile/internal/pattern"
	"github.com/reconcile/internal/pipeline"
	"github.com/reconcile/internal/rules"
	"github.com/reconcile/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		runGenerate(os.Args[2:])
	case "run":
		runReconcile(os.Args[2:])
	case "benchmark":
		runBenchmark(os.Args[2:])
	case "agent":
		runAgent(os.Args[2:])
	case "why":
		runWhy(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	cmd := flag.NewFlagSet("generate", flag.ExitOnError)
	outputDir := cmd.String("out", "testdata", "directory to write generated CSV files")
	cmd.Parse(args)

	if err := ingest.GenerateFixtures(*outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "error generating fixtures: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ test fixtures written to %s/\n", *outputDir)
}

func runReconcile(args []string) {
	cmd := flag.NewFlagSet("run", flag.ExitOnError)
	rulesPath := cmd.String("rules", "rules.yaml", "path to rules.yaml configuration")
	exactOnly := cmd.Bool("exact-only", false, "run exact match pass only (skip fuzzy)")
	auditPath := cmd.String("audit", "audit.jsonl", "path for JSONL audit trail")
	htmlPath := cmd.String("html", "", "path for HTML report (optional)")
	csvExcPath := cmd.String("csv-exceptions", "", "path for exceptions CSV export (optional)")
	csvMatchPath := cmd.String("csv-matches", "", "path for matches CSV export (optional)")
	jsonPath := cmd.String("json", "", "path for JSON summary export (optional)")
	timeoutSec := cmd.Int("timeout", 30, "pipeline timeout in seconds")
	cmd.Parse(args)

	cfg, err := rules.Load(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	start := time.Now()
	result, err := pipeline.Run(ctx, cfg, pipeline.Options{ExactOnly: *exactOnly})
	if err != nil {
		fmt.Fprintf(os.Stderr, "reconciliation failed: %v\n", err)
		os.Exit(1)
	}
	elapsed := time.Since(start)

	summary := metrics.Compute(result)
	metrics.PrintSummary(os.Stdout, summary)

	alerts := pattern.Detect(result.Exceptions, 3)
	metrics.PrintAlerts(os.Stdout, alerts)

	fmt.Printf("Execution time: %v\n", elapsed)

	if *auditPath != "" {
		lines, err := audit.WriteAuditLog(*auditPath, result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: audit log failed: %v\n", err)
		} else {
			fmt.Printf("✓ audit log  → %s (%d entries)\n", *auditPath, lines)
		}
	}

	if *csvExcPath != "" {
		n, err := export.WriteExceptionsCSV(*csvExcPath, result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: exceptions CSV failed: %v\n", err)
		} else {
			fmt.Printf("✓ exceptions → %s (%d rows)\n", *csvExcPath, n)
		}
	}

	if *csvMatchPath != "" {
		n, err := export.WriteMatchesCSV(*csvMatchPath, result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: matches CSV failed: %v\n", err)
		} else {
			fmt.Printf("✓ matches    → %s (%d rows)\n", *csvMatchPath, n)
		}
	}

	if *htmlPath != "" {
		if err := export.WriteHTMLReport(*htmlPath, result, summary); err != nil {
			fmt.Fprintf(os.Stderr, "warning: HTML report failed: %v\n", err)
		} else {
			fmt.Printf("✓ HTML report → %s\n", *htmlPath)
		}
	}

	if *jsonPath != "" {
		if err := export.ExportJSON(*jsonPath, result, summary); err != nil {
			fmt.Fprintf(os.Stderr, "warning: JSON export failed: %v\n", err)
		} else {
			fmt.Printf("✓ JSON summary → %s\n", *jsonPath)
		}
	}

	fmt.Println()
}

func runBenchmark(args []string) {
	cmd := flag.NewFlagSet("benchmark", flag.ExitOnError)
	numRecords := cmd.Int("records", 10000, "number of transaction groups to generate")
	rulesPath := cmd.String("rules", "rules.yaml", "path to rules.yaml configuration")
	noCleanup := cmd.Bool("no-cleanup", false, "keep the generated benchmark files after run")
	cmd.Parse(args)

	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("reconcile-bench-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	if !*noCleanup {
		defer os.RemoveAll(tmpDir)
	} else {
		fmt.Printf("benchmark fixtures at: %s\n", tmpDir)
	}

	fmt.Printf("\nGenerating %d transaction groups...\n", *numRecords)
	genStart := time.Now()
	if err := ingest.GenerateLargeFixtures(tmpDir, *numRecords); err != nil {
		fmt.Fprintf(os.Stderr, "fixture generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ generated in %v\n\n", time.Since(genStart).Round(time.Millisecond))

	cfg, err := rules.Load(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rules load failed: %v\n", err)
		os.Exit(1)
	}

	// Point sources at the benchmark files
	for i := range cfg.Sources {
		switch cfg.Sources[i].Name {
		case "gateway":
			cfg.Sources[i].File = filepath.Join(tmpDir, "gateway_settlement.csv")
		case "bank":
			cfg.Sources[i].File = filepath.Join(tmpDir, "bank_statement.csv")
		case "ledger":
			cfg.Sources[i].File = filepath.Join(tmpDir, "internal_ledger.csv")
		}
	}

	ctx := context.Background()
	start := time.Now()
	result, err := pipeline.Run(ctx, cfg, pipeline.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipeline failed: %v\n", err)
		os.Exit(1)
	}
	elapsed := time.Since(start)

	summary := metrics.Compute(result)
	metrics.PrintSummary(os.Stdout, summary)

	totalRecords := result.TotalRecordsRead
	throughput := float64(totalRecords) / elapsed.Seconds()
	fmt.Printf("  Wall time      : %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Throughput     : %.0f records/sec\n\n", throughput)
}

func runAgent(args []string) {
	cmd := flag.NewFlagSet("agent", flag.ExitOnError)
	rulesPath := cmd.String("rules", "rules.yaml", "path to rules.yaml configuration")
	outPath := cmd.String("out", "resolution_report.md", "path to save AI report")
	cmd.Parse(args)

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: GEMINI_API_KEY environment variable is required.")
		fmt.Fprintln(os.Stderr, "Get one at https://aistudio.google.com/app/apikey")
		os.Exit(1)
	}

	fmt.Println("Running reconciliation pipeline to gather exceptions...")
	cfg, err := rules.Load(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rules load failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	result, err := pipeline.Run(ctx, cfg, pipeline.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipeline failed: %v\n", err)
		os.Exit(1)
	}

	if len(result.Exceptions) == 0 {
		fmt.Println("No exceptions found! Nothing for the AI agent to do.")
		return
	}

	fmt.Printf("Found %d exceptions. Pinging AI Finance Controller (Gemini)...\n", len(result.Exceptions))
	
	report, err := agent.GenerateResolutionReport(ctx, apiKey, result.Exceptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AI agent failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outPath, []byte(report), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save report: %v\n", err)
	}

	fmt.Printf("\n================ AI RESOLUTION REPORT ================\n\n")
	fmt.Println(report)
	fmt.Printf("\n======================================================\n")
	fmt.Printf("✓ Saved full report to %s\n", *outPath)
}

func printUsage() {
	fmt.Println("reconcile — multi-source financial reconciliation tool")
	fmt.Println("\nUsage:")
	fmt.Println("  reconcile <command> [flags]")
	fmt.Println("\nCommands:")
	fmt.Println("  generate          generate synthetic test fixtures")
	fmt.Println("    -out <dir>        output directory (default: testdata)")
	fmt.Println()
	fmt.Println("  run               run the full reconciliation pipeline")
	fmt.Println("    -rules <file>     rules YAML path (default: rules.yaml)")
	fmt.Println("    -exact-only       skip fuzzy pass")
	fmt.Println("    -audit <file>     JSONL audit trail (default: audit.jsonl)")
	fmt.Println("    -html <file>      HTML report (optional)")
	fmt.Println("    -csv-exceptions   exceptions CSV export (optional)")
	fmt.Println("    -csv-matches      matches CSV export (optional)")
	fmt.Println("    -json <file>      JSON summary export (optional)")
	fmt.Println("    -timeout <sec>    pipeline timeout (default: 30)")
	fmt.Println()
	fmt.Println("  benchmark         stress test with large synthetic datasets")
	fmt.Println("    -records <n>      number of transaction groups (default: 10000)")
	fmt.Println("    -rules <file>     rules YAML path (default: rules.yaml)")
	fmt.Println("    -no-cleanup       keep generated files after run")
	fmt.Println()
	fmt.Println("  agent             use AI to resolve reconciliation exceptions (requires GEMINI_API_KEY)")
	fmt.Println("    -rules <file>     rules YAML path (default: rules.yaml)")
	fmt.Println("    -out <file>       path to save AI report (default: resolution_report.md)")
	fmt.Println()
	fmt.Println("  why               ask AI to explain a specific record in plain English (requires GEMINI_API_KEY)")
	fmt.Println("    -record <id>      the record ID to look up (e.g. GW-20)")
	fmt.Println("    -audit <file>     audit log to read from (default: audit.jsonl)")
	fmt.Println()
	fmt.Println("  serve             start the web UI (optional: requires GEMINI_API_KEY for AI features)")
	fmt.Println("    -port <port>      port to listen on (default: 8080)")
}

func runWhy(args []string) {
	cmd := flag.NewFlagSet("why", flag.ExitOnError)
	recordID := cmd.String("record", "", "record ID to explain (required)")
	auditPath := cmd.String("audit", "audit.jsonl", "path to audit.jsonl")
	cmd.Parse(args)

	if *recordID == "" {
		fmt.Fprintln(os.Stderr, "Error: -record <id> is required.")
		fmt.Fprintln(os.Stderr, "Example: reconcile why -record GW-20")
		os.Exit(1)
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: GEMINI_API_KEY environment variable is required.")
		fmt.Fprintln(os.Stderr, "Get one at https://aistudio.google.com/app/apikey")
		os.Exit(1)
	}

	ctx := context.Background()
	explanation, err := agent.ExplainRecord(ctx, apiKey, *auditPath, *recordID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[ AI explanation for record %s ]\n\n", *recordID)
	fmt.Println(explanation)
	fmt.Println()
}

func runServe(args []string) {
	cmd := flag.NewFlagSet("serve", flag.ExitOnError)
	port := cmd.String("port", "8080", "port to serve on")
	cmd.Parse(args)

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Warning: GEMINI_API_KEY is not set. AI features in the UI will be disabled.")
	}

	srv := server.New(apiKey)
	if err := srv.Start(":" + *port); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
