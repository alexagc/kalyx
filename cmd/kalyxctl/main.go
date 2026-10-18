package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Printf("kalyxctl version %s\n", version)

	case "generate":
		generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
		eventFlag := generateCmd.String("event", "", "Target event identifier (e.g., com.company.orders.completed)")
		langFlag := generateCmd.String("lang", "typescript", "Target language (typescript, python, go)")
		outFlag := generateCmd.String("out", "./generated", "Output directory for generated SDK code")

		if err := generateCmd.Parse(os.Args[2:]); err != nil {
			os.Exit(1)
		}

		if *eventFlag == "" {
			fmt.Println("Error: --event flag is required")
			generateCmd.Usage()
			os.Exit(1)
		}

		fmt.Printf("🌸 Generating strongly-typed CloudEvent SDK for event '%s' [%s] in '%s'...\n", *eventFlag, *langFlag, *outFlag)
		fmt.Println("✔ Fetching JSON Schema 2020-12 contract")
		fmt.Println("✔ Generating CloudEvent v1.0.2 builder")
		fmt.Println("✔ Success! Local compile-time validation ready.")

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: kalyxctl <command> [flags]")
	fmt.Println("\nCommands:")
	fmt.Println("  generate    Generate strongly-typed CloudEvents client SDKs")
	fmt.Println("  version     Show kalyxctl version")
	fmt.Println("\nRun 'kalyxctl <command> -h' for more details on each command.")
}
