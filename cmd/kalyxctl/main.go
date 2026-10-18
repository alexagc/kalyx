package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexagc/kalyx/internal/cli"
	"github.com/alexagc/kalyx/internal/cli/codegen"
)

const version = "0.1.0"

func main() {
	exitCode := Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// Run executes the kalyxctl command-line application with injected I/O streams for testing.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 1
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "kalyxctl version %s\n", version)
		return 0

	case "login":
		return runLogin(cmdArgs, stdin, stdout, stderr)

	case "generate":
		return runGenerate(cmdArgs, stdout, stderr)

	case "help", "--help", "-h":
		printUsage(stdout)
		return 0

	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", command)
		printUsage(stderr)
		return 1
	}
}

func runLogin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)

	tokenFlag := fs.String("token", "", "Personal Access Token (PAT)")
	serverFlag := fs.String("server", "http://localhost:8080", "Kalyx server URL")
	profileFlag := fs.String("profile", "default", "Profile name in ~/.kalyx/config.yaml")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		fmt.Fprint(stdout, "Enter Personal Access Token (PAT): ")
		scanner := bufio.NewScanner(stdin)
		if scanner.Scan() {
			token = strings.TrimSpace(scanner.Text())
		}
	}

	if token == "" {
		fmt.Fprintln(stderr, "Error: Personal Access Token (PAT) cannot be empty")
		return 1
	}

	globalPath, err := cli.DefaultGlobalConfigPath()
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving config path: %v\n", err)
		return 1
	}

	profile := *profileFlag
	if profile == "" {
		profile = "default"
	}

	if err := cli.SaveToken(globalPath, profile, *serverFlag, token); err != nil {
		fmt.Fprintf(stderr, "Error saving token: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "✔ Successfully authenticated with Kalyx! Token saved to profile '%s' in %s\n", profile, globalPath)
	return 0
}

func runGenerate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	eventFlag := fs.String("event", "", "Target event identifier (e.g., com.company.orders.completed.v1)")
	langFlag := fs.String("lang", "", "Target language (typescript, python, go)")
	outFlag := fs.String("out", "", "Output directory for generated SDK code")
	serverFlag := fs.String("server", "", "Kalyx server URL")
	tokenFlag := fs.String("token", "", "Personal Access Token (PAT)")
	profileFlag := fs.String("profile", "", "Active profile in ~/.kalyx/config.yaml")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	flags := cli.ConfigFlags{
		Event:   *eventFlag,
		Lang:    *langFlag,
		Out:     *outFlag,
		Server:  *serverFlag,
		Token:   *tokenFlag,
		Profile: *profileFlag,
	}

	effective, err := cli.ResolveConfig(flags, "", "")
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving configuration: %v\n", err)
		return 1
	}

	if len(effective.Events) == 0 {
		fmt.Fprintln(stderr, "Error: No events specified. Use --event <type> or configure events list in .kalyx.yaml")
		return 1
	}

	lang := strings.ToLower(effective.Lang)
	if lang == "" {
		lang = "typescript"
	}

	outDir := effective.Out
	if outDir == "" {
		outDir = "./generated"
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(stderr, "Error creating output directory %s: %v\n", outDir, err)
		return 1
	}

	client := cli.NewClient(effective.Server, effective.Token)
	ctx := context.Background()

	fmt.Fprintf(stdout, "🌸 Kalyx Code Generator (%s -> %s)\n", lang, outDir)

	for _, eventType := range effective.Events {
		fmt.Fprintf(stdout, "• Fetching contract '%s' from %s...\n", eventType, effective.Server)

		contract, err := client.FetchContract(ctx, eventType)
		if err != nil {
			fmt.Fprintf(stderr, "Error: Failed to fetch contract '%s': %v\n", eventType, err)
			return 1
		}

		if contract.Deprecated {
			sunset := contract.SunsetDate
			if sunset == "" {
				sunset = "immediate"
			}
			fmt.Fprintf(stdout, "⚠️  WARNING [RFC 8594]: Contract '%s' (v%s) is DEPRECATED. Sunset date: %s. Please migrate to a newer contract version.\n", contract.Type, contract.SemVer, sunset)
		}

		var code string
		var ext string
		eventName := codegen.DeriveEventName(contract.Name, contract.Type)
		fileName := codegen.ToSnakeCase(eventName)

		switch lang {
		case "typescript", "ts":
			ext = ".ts"
			code, err = codegen.GenerateTypeScript(*contract)
		case "python", "py":
			ext = ".py"
			code, err = codegen.GeneratePython(*contract)
		case "go", "golang":
			ext = ".go"
			code, err = codegen.GenerateGo(*contract, "events")
		default:
			fmt.Fprintf(stderr, "Error: Unsupported language '%s'. Supported: typescript, python, go\n", lang)
			return 1
		}

		if err != nil {
			fmt.Fprintf(stderr, "Error generating %s code for '%s': %v\n", lang, eventType, err)
			return 1
		}

		targetFile := filepath.Join(outDir, fileName+ext)
		if err := os.WriteFile(targetFile, []byte(code), 0644); err != nil {
			fmt.Fprintf(stderr, "Error writing generated code to %s: %v\n", targetFile, err)
			return 1
		}

		fmt.Fprintf(stdout, "✔ Generated %s SDK file: %s\n", lang, targetFile)
	}

	fmt.Fprintln(stdout, "✔ Code generation completed successfully.")
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: kalyxctl <command> [flags]")
	fmt.Fprintln(w, "\nCommands:")
	fmt.Fprintln(w, "  login       Authenticate kalyxctl with a Personal Access Token (PAT)")
	fmt.Fprintln(w, "  generate    Generate strongly-typed CloudEvents client SDKs")
	fmt.Fprintln(w, "  version     Show kalyxctl version")
	fmt.Fprintln(w, "\nRun 'kalyxctl <command> -h' for more details on each command.")
}
