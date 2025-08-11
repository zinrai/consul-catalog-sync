package main

import (
	"log"
)

var (
	version    = "0.1.0"
	binaryName = "consul-catalog-sync"
)

func main() {
	// Parse and validate configuration
	config := parseConfig()
	setupLogging(config)

	// Load vars (file or directory)
	varsData, err := loadVars(config.VarsPath, config.Verbose)
	if err != nil {
		log.Fatalf("[ERROR] Failed to load vars: %v", err)
	}

	// Load mapping
	mappingConfig, err := loadMapping(config.MappingFile)
	if err != nil {
		log.Fatalf("[ERROR] Failed to load mapping: %v", err)
	}

	// Generate operations for all nodes
	operations := generateAllOperations(varsData, mappingConfig, config.Datacenter)

	// Execute based on mode
	executeMode(config, operations)
}

func generateAllOperations(varsData map[string]interface{}, mappingConfig *MappingConfig, datacenter string) []map[string]interface{} {
	log.Printf("[INFO] Generating operations for %d nodes", len(varsData))
	allOperations := []map[string]interface{}{}

	for key, value := range varsData {
		nodeValue, ok := value.(map[string]interface{})
		if !ok {
			log.Printf("[WARN] Skipping invalid node: %s", key)
			continue
		}

		ctx := ExecutionContext{
			Key:        key,
			Value:      nodeValue,
			Datacenter: datacenter,
		}

		operations, err := GenerateOperations(ctx, mappingConfig)
		if err != nil {
			log.Printf("[ERROR] Failed to generate operations for %s: %v", key, err)
			continue
		}

		allOperations = append(allOperations, operations...)
	}

	log.Printf("[INFO] Generated %d operations", len(allOperations))
	return allOperations
}

func executeMode(config Config, operations []map[string]interface{}) {
	// Output payload if requested
	if config.Payload {
		outputPayload(operations, config.Datacenter, config.Verbose)
		return
	}

	// Dry-run mode
	if config.DryRun {
		printDryRun(operations, config.Verbose)
		return
	}

	// Execute operations
	err := ExecuteOperations(config.ConsulAddr, operations, config.Verbose)
	if err != nil {
		log.Fatalf("[ERROR] Failed to execute operations: %v", err)
	}
	log.Printf("[INFO] Successfully synced %d operations", len(operations))
}
