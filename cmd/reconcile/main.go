package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/reconcile/internal/audit"
	"github.com/reconcile/internal/ingest"
	"github.com/reconcile/internal/metrics"
	"github.com/reconcile/internal/pipeline"
	"github.com/reconcile/internal/rules"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate":
		generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
		outputDir := generateCmd.String("out", "testdata", "Directory to write generated CSV files")
		if err := generateCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		err := ingest.GenerateFixtures(*outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating fixtures: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Synthetic test fixtures successfully generated in %s/\n", *outputDir)

	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		rulesPath := runCmd.String("rules", "rules.yaml", "Path to rules.yaml configuration")
		exactOnly := runCmd.Bool("exact-only", false, "Run exact match pass only")
		auditPath := runCmd.String("audit", "audit.jsonl", "Path to write JSONL audit log")
		timeoutSec := runCmd.Int("timeout", 30, "Pipeline execution timeout in seconds")

		if err := runCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		cfg, err := rules.Load(*rulesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
		defer cancel()

		opts := pipeline.Options{
			ExactOnly: *exactOnly,
		}

		startTime := time.Now()
		result, err := pipeline.Run(ctx, cfg, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Reconciliation failed: %v\n", err)
			os.Exit(1)
		}
		duration := time.Since(startTime)

		summary := metrics.Compute(result)
		metrics.PrintSummary(os.Stdout, summary)
		fmt.Printf("Execution time: %v\n", duration)

		if *auditPath != "" {
			lines, err := audit.WriteAuditLog(*auditPath, result)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed writing audit log: %v\n", err)
			} else {
				fmt.Printf("✓ Audit log successfully written to %s (%d records logged)\n\n", *auditPath, lines)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Multi-Source Financial Reconciliation CLI")
	fmt.Println("\nUsage:")
	fmt.Println("  reconcile <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  generate              Generate synthetic multi-source test fixtures")
	fmt.Println("    -out <dir>          Output directory (default: testdata)")
	fmt.Println("\n  run                   Run reconciliation pipeline")
	fmt.Println("    -rules <file>       Path to rules YAML (default: rules.yaml)")
	fmt.Println("    -exact-only         Run exact-match pass only")
	fmt.Println("    -audit <file>       Path for JSONL audit trail (default: audit.jsonl)")
	fmt.Println("    -timeout <sec>      Pipeline timeout in seconds (default: 30)")
}
