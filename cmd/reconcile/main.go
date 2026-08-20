package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/reconcile/internal/ingest"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: reconcile <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  generate    Generate synthetic test data")
		fmt.Println("  run         Run reconciliation (not implemented yet)")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate":
		generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
		outputDir := generateCmd.String("out", "testdata", "Directory to write CSV files")
		generateCmd.Parse(os.Args[2:])

		err := ingest.GenerateFixtures(*outputDir)
		if err != nil {
			fmt.Printf("Error generating fixtures: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Fixtures successfully generated in %s/\n", *outputDir)

	case "run":
		fmt.Println("Run command is not fully implemented yet.")
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
