package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/alexagc/kalyx/internal/cli"
	"github.com/alexagc/kalyx/internal/cli/codegen"
	"github.com/alexagc/kalyx/internal/core"
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

	case "list":
		return runList(cmdArgs, stdout, stderr)

	case "validate":
		return runValidate(cmdArgs, stdout, stderr)

	case "check":
		return runCheck(cmdArgs, stdout, stderr)

	case "generate":
		return runGenerate(cmdArgs, stdout, stderr)

	case "token":
		return runToken(cmdArgs, stdout, stderr)

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

	serverURL := strings.TrimRight(*serverFlag, "/")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Active authentication handshake
	client := cli.NewClient(serverURL, token)
	ctx := context.Background()
	user, err := client.GetMe(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "Error: Failed to authenticate with Kalyx server: %v\n", err)
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

	if err := cli.SaveToken(globalPath, profile, serverURL, token); err != nil {
		fmt.Fprintf(stderr, "Error saving token: %v\n", err)
		return 1
	}

	teamsStr := strings.Join(user.Teams, ", ")
	if teamsStr == "" {
		teamsStr = "no team"
	}

	fmt.Fprintf(stdout, "✔ Successfully authenticated as %s (%s) with Kalyx! Token saved to profile '%s' in %s\n", user.Email, teamsStr, profile, globalPath)
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

func splitFlagsAndPositional(args []string, flagsWithArgs map[string]bool) (flags []string, pos []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if !strings.Contains(arg, "=") && flagsWithArgs[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			pos = append(pos, arg)
		}
	}
	return flags, pos
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)

	domainFlag := fs.String("domain", "", "Filter events by domain name")
	jsonFlag := fs.Bool("json", false, "Output results in JSON format")
	serverFlag := fs.String("server", "", "Kalyx server URL")
	tokenFlag := fs.String("token", "", "Personal Access Token (PAT)")
	profileFlag := fs.String("profile", "", "Active profile in ~/.kalyx/config.yaml")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	flags := cli.ConfigFlags{
		Server:  *serverFlag,
		Token:   *tokenFlag,
		Profile: *profileFlag,
	}
	effective, err := cli.ResolveConfig(flags, "", "")
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving configuration: %v\n", err)
		return 1
	}

	client := cli.NewClient(effective.Server, effective.Token)
	ctx := context.Background()

	events, err := client.ListEvents(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "Error listing events: %v\n", err)
		return 1
	}

	domain := strings.TrimSpace(*domainFlag)
	var filtered []core.EventDefinition
	for _, ev := range events {
		if domain == "" || strings.EqualFold(ev.Domain, domain) {
			filtered = append(filtered, ev)
		}
	}

	if *jsonFlag {
		if filtered == nil {
			filtered = []core.EventDefinition{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(filtered); err != nil {
			fmt.Fprintf(stderr, "Error encoding JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tDOMAIN\tOWNER TEAM")
	for _, ev := range filtered {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ev.ID, ev.Name, ev.Type, ev.Domain, ev.OwnerTeam)
	}
	w.Flush()
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	schemaFlag := fs.String("schema", "", "Path to contract JSON Schema file")
	eventFlag := fs.String("event", "", "Target event identifier or type")
	serverFlag := fs.String("server", "", "Kalyx server URL")
	tokenFlag := fs.String("token", "", "Personal Access Token (PAT)")
	profileFlag := fs.String("profile", "", "Active profile in ~/.kalyx/config.yaml")

	flagArgs, posArgs := splitFlagsAndPositional(args, map[string]bool{
		"schema":  true,
		"event":   true,
		"server":  true,
		"token":   true,
		"profile": true,
	})

	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	if len(posArgs) == 0 {
		fmt.Fprintln(stderr, "Error: Event JSON file path is required. Usage: kalyxctl validate <event.json> [flags]")
		return 1
	}

	eventFile := posArgs[0]
	eventBytes, err := os.ReadFile(eventFile)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading event file '%s': %v\n", eventFile, err)
		return 1
	}

	var schemaStr string
	if *schemaFlag != "" {
		schemaBytes, err := os.ReadFile(*schemaFlag)
		if err != nil {
			fmt.Fprintf(stderr, "Error reading schema file '%s': %v\n", *schemaFlag, err)
			return 1
		}
		schemaStr = string(schemaBytes)
	}

	flags := cli.ConfigFlags{
		Event:   *eventFlag,
		Server:  *serverFlag,
		Token:   *tokenFlag,
		Profile: *profileFlag,
	}
	effective, err := cli.ResolveConfig(flags, "", "")
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving configuration: %v\n", err)
		return 1
	}

	client := cli.NewClient(effective.Server, effective.Token)
	ctx := context.Background()

	targetEvent := ""
	if len(effective.Events) > 0 {
		targetEvent = effective.Events[0]
	}

	// If schema was not explicitly provided via --schema, but --event was specified, fetch latest contract schema
	if schemaStr == "" && targetEvent != "" {
		events, err := client.ListEvents(ctx)
		if err == nil {
			for _, ev := range events {
				if ev.Type == targetEvent || ev.ID == targetEvent || ev.Name == targetEvent {
					versions, vErr := client.ListVersions(ctx, ev.ID)
					if vErr == nil && len(versions) > 0 {
						schemaStr = versions[len(versions)-1].JSONSchema
					}
					break
				}
			}
		}
	}

	res, err := client.ValidateEvent(ctx, eventBytes, schemaStr)
	if err != nil {
		fmt.Fprintf(stderr, "Error validating CloudEvent: %v\n", err)
		return 1
	}

	if !res.Valid {
		fmt.Fprintln(stderr, "❌ CloudEvent validation failed against contract schema:")
		for _, e := range res.Errors {
			fmt.Fprintf(stderr, "  • %s\n", e)
		}
		return 1
	}

	fmt.Fprintln(stdout, "✔ CloudEvent is valid against contract schema")
	return 0
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)

	serverFlag := fs.String("server", "", "Kalyx server URL")
	tokenFlag := fs.String("token", "", "Personal Access Token (PAT)")
	profileFlag := fs.String("profile", "", "Active profile in ~/.kalyx/config.yaml")

	flagArgs, posArgs := splitFlagsAndPositional(args, map[string]bool{
		"server":  true,
		"token":   true,
		"profile": true,
	})

	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	if len(posArgs) < 2 {
		fmt.Fprintln(stderr, "Error: Both old and new schema JSON files are required. Usage: kalyxctl check <old_schema.json> <new_schema.json> [flags]")
		return 1
	}

	oldFile := posArgs[0]
	newFile := posArgs[1]

	oldBytes, err := os.ReadFile(oldFile)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading old schema file '%s': %v\n", oldFile, err)
		return 1
	}

	newBytes, err := os.ReadFile(newFile)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading new schema file '%s': %v\n", newFile, err)
		return 1
	}

	flags := cli.ConfigFlags{
		Server:  *serverFlag,
		Token:   *tokenFlag,
		Profile: *profileFlag,
	}
	effective, err := cli.ResolveConfig(flags, "", "")
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving configuration: %v\n", err)
		return 1
	}

	client := cli.NewClient(effective.Server, effective.Token)
	ctx := context.Background()

	res, err := client.CheckCompatibility(ctx, string(oldBytes), string(newBytes))
	if err != nil {
		fmt.Fprintf(stderr, "Error checking schema compatibility: %v\n", err)
		return 1
	}

	reasons := strings.Join(res.Reasons, "; ")
	if len(res.Reasons) == 0 {
		reasons = "No changes detected"
	}

	if res.IsCompatible {
		fmt.Fprintf(stdout, "✔ Schemas are backward compatible (Change Type: %s)\nReasons: %s\n", res.ChangeType, reasons)
		return 0
	}

	fmt.Fprintf(stderr, "❌ BREAKING SCHEMA CHANGE DETECTED (Change Type: %s)\nReasons: %s\n", res.ChangeType, reasons)
	return 1
}

func runToken(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: kalyxctl token <subcommand> [flags]\n\nSubcommands:\n  create    Create a new Personal Access Token (PAT)")
		return 1
	}

	subcommand := args[0]
	cmdArgs := args[1:]

	switch subcommand {
	case "create":
		return runTokenCreate(cmdArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown token subcommand: %s\n", subcommand)
		fmt.Fprintln(stderr, "Usage: kalyxctl token create --name <name> [flags]")
		return 1
	}
}

func runTokenCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("token create", flag.ContinueOnError)
	fs.SetOutput(stderr)

	nameFlag := fs.String("name", "", "Name for the Personal Access Token")
	expiresInFlag := fs.Int("expires-in", 30, "Token expiration in days (default: 30)")
	serverFlag := fs.String("server", "", "Kalyx server URL")
	tokenFlag := fs.String("token", "", "Personal Access Token (PAT)")
	profileFlag := fs.String("profile", "", "Active profile in ~/.kalyx/config.yaml")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	name := strings.TrimSpace(*nameFlag)
	if name == "" {
		fmt.Fprintln(stderr, "Error: Token name is required (--name <name>)")
		return 1
	}

	flags := cli.ConfigFlags{
		Server:  *serverFlag,
		Token:   *tokenFlag,
		Profile: *profileFlag,
	}
	effective, err := cli.ResolveConfig(flags, "", "")
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving configuration: %v\n", err)
		return 1
	}

	client := cli.NewClient(effective.Server, effective.Token)
	ctx := context.Background()

	pat, err := client.CreatePAT(ctx, name, *expiresInFlag)
	if err != nil {
		fmt.Fprintf(stderr, "Error creating Personal Access Token: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "✔ Personal Access Token '%s' created successfully!\n", pat.Name)
	fmt.Fprintf(stdout, "\nToken: %s\n", pat.Token)
	fmt.Fprintln(stdout, "\n⚠️  WARNING: Store this token securely. It will not be displayed again.")
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: kalyxctl <command> [flags]")
	fmt.Fprintln(w, "\nCommands:")
	fmt.Fprintln(w, "  login       Authenticate kalyxctl with a Personal Access Token (PAT)")
	fmt.Fprintln(w, "  list        List registered event contracts")
	fmt.Fprintln(w, "  validate    Validate a CloudEvent JSON file against a contract schema")
	fmt.Fprintln(w, "  check       Check backward compatibility between two JSON schemas")
	fmt.Fprintln(w, "  generate    Generate strongly-typed CloudEvents client SDKs")
	fmt.Fprintln(w, "  token       Manage Personal Access Tokens (PATs)")
	fmt.Fprintln(w, "  version     Show kalyxctl version")
	fmt.Fprintln(w, "\nRun 'kalyxctl <command> -h' for more details on each command.")
}
