package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

// Config holds all command-line configuration
type Config struct {
	VarsPath    string
	MappingFile string
	Datacenter  string
	ConsulAddr  string
	DryRun      bool
	Verbose     bool
	Payload     bool
}

func parseConfig() Config {
	config := parseFlags()

	// Handle version and help flags
	if handleSpecialFlags(config) {
		os.Exit(0)
	}

	// Validate required flags
	if !validateRequiredFlags(config) {
		showUsage()
		os.Exit(1)
	}

	return config
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.VarsPath, "vars", "", "vars file or directory path (required)")
	flag.StringVar(&config.MappingFile, "mapping", "", "mapping file path (required)")
	flag.StringVar(&config.Datacenter, "datacenter", "dc1", "target datacenter (default: dc1)")
	flag.StringVar(&config.ConsulAddr, "consul-addr", "http://127.0.0.1:8500", "Consul HTTP address")
	flag.BoolVar(&config.DryRun, "dry-run", false, "show operations without executing")
	flag.BoolVar(&config.Verbose, "verbose", false, "verbose output")
	flag.BoolVar(&config.Payload, "payload", false, "output JSON payload that would be sent to Consul API (NDJSON format)")

	flag.Parse()

	return config
}

func handleSpecialFlags(config Config) bool {
	// Check for version flag
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Printf("%s version %s\n", binaryName, version)
		return true
	}

	// Check for help flag
	if len(os.Args) > 1 && os.Args[1] == "-help" {
		showUsage()
		return true
	}

	return false
}

func validateRequiredFlags(config Config) bool {
	return config.VarsPath != "" && config.MappingFile != ""
	// datacenter now has a default value, so it's not required
}

func showUsage() {
	fmt.Fprintf(os.Stderr, "%s - Sync node and service definitions to Consul Catalog\n\n", binaryName)
	fmt.Fprintf(os.Stderr, "Version: %s\n\n", version)
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s -vars <path> -mapping <file> [options]\n\n", binaryName)
	fmt.Fprintf(os.Stderr, "Required flags:\n")
	fmt.Fprintf(os.Stderr, "  -vars        Path to vars file or directory containing YAML files\n")
	fmt.Fprintf(os.Stderr, "  -mapping     Path to mapping rules file\n\n")
	fmt.Fprintf(os.Stderr, "Optional flags:\n")
	fmt.Fprintf(os.Stderr, "  -datacenter  Target datacenter (default: dc1)\n")
	fmt.Fprintf(os.Stderr, "  -consul-addr Consul HTTP address (default: http://127.0.0.1:8500)\n")
	fmt.Fprintf(os.Stderr, "  -dry-run     Show operations without executing\n")
	fmt.Fprintf(os.Stderr, "  -verbose     Verbose output\n")
	fmt.Fprintf(os.Stderr, "  -payload     Output JSON payload (NDJSON format)\n")
	fmt.Fprintf(os.Stderr, "  -version     Show version\n")
	fmt.Fprintf(os.Stderr, "  -help        Show this help message\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  # Sync from single file (uses default datacenter: dc1)\n")
	fmt.Fprintf(os.Stderr, "  %s -vars nodes.yaml -mapping mapping.yaml\n\n", binaryName)
	fmt.Fprintf(os.Stderr, "  # Sync from directory with specific datacenter\n")
	fmt.Fprintf(os.Stderr, "  %s -vars vars/ -mapping mapping.yaml -datacenter prod\n\n", binaryName)
	fmt.Fprintf(os.Stderr, "  # Use custom Consul address\n")
	fmt.Fprintf(os.Stderr, "  %s -vars vars/ -mapping mapping.yaml -consul-addr http://consul.example.com:8500\n\n", binaryName)
	fmt.Fprintf(os.Stderr, "  # Dry run to see what would be synced\n")
	fmt.Fprintf(os.Stderr, "  %s -vars vars/ -mapping mapping.yaml -dry-run\n\n", binaryName)
	fmt.Fprintf(os.Stderr, "  # Output JSON payload for debugging\n")
	fmt.Fprintf(os.Stderr, "  %s -vars vars/ -mapping mapping.yaml -payload | jq '.'\n\n", binaryName)
}

func setupLogging(config Config) {
	// Suppress INFO logs when outputting payload (they would corrupt JSON output)
	if config.Payload {
		// Set log output to stderr for warnings and errors only
		log.SetOutput(os.Stderr)
		log.SetPrefix("[WARN/ERROR] ")
		log.SetFlags(0)

		// Create a custom logger that filters INFO messages
		// For now, we'll just redirect all logs to stderr
		return
	}

	if config.Verbose {
		log.SetFlags(log.Ltime | log.Lmicroseconds)
	} else {
		log.SetFlags(0)
	}
}

// Helper function to suppress stdout logs when in payload mode
func logInfo(format string, args ...interface{}) {
	// This can be called instead of log.Printf for INFO level logs
	// It will be suppressed in payload mode
	if !isPayloadMode() {
		log.Printf("[INFO] "+format, args...)
	}
}

func isPayloadMode() bool {
	// Check if we're in payload mode by checking flags
	for _, arg := range os.Args {
		if arg == "-payload" || arg == "--payload" {
			return true
		}
	}
	return false
}

// Disable stdout for INFO logs when in payload mode
func init() {
	if isPayloadMode() {
		// Custom log handling for payload mode
		log.SetOutput(io.Discard)
	}
}
