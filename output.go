package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// printDryRun outputs human-readable dry-run information
func printDryRun(operations []map[string]interface{}, verbose bool) {
	fmt.Println("=== DRY RUN MODE ===")
	fmt.Printf("Total operations: %d\n", len(operations))

	// Count operation types
	counts := countOperationTypes(operations)
	fmt.Printf("- Node operations: %d\n", counts["node"])
	fmt.Printf("- Service operations: %d\n", counts["service"])
	if counts["check"] > 0 {
		fmt.Printf("- Check operations: %d\n", counts["check"])
	}

	// Calculate batches
	const maxBatchSize = 64
	batchCount := (len(operations) + maxBatchSize - 1) / maxBatchSize
	fmt.Printf("Batches required: %d (%d operations per batch max)\n", batchCount, maxBatchSize)

	if verbose {
		printOperationsDetail(operations)
	}
}

// outputPayload outputs operations as NDJSON (one line per batch)
func outputPayload(operations []map[string]interface{}, datacenter string, verbose bool) {
	const maxBatchSize = 64
	totalBatches := (len(operations) + maxBatchSize - 1) / maxBatchSize

	for i := 0; i < len(operations); i += maxBatchSize {
		end := min(i+maxBatchSize, len(operations))
		batch := operations[i:end]
		batchNum := (i / maxBatchSize) + 1

		// Create batch object
		batchObj := map[string]interface{}{
			"batch":      batchNum,
			"size":       len(batch),
			"operations": batch,
		}

		// Add verbose information if requested
		if verbose {
			batchObj["total_batches"] = totalBatches
			batchObj["max_batch_size"] = maxBatchSize
			batchObj["datacenter"] = datacenter
		}

		// Output as NDJSON (one line per batch)
		jsonBytes, err := json.Marshal(batchObj)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to marshal batch %d: %v\n", batchNum, err)
			continue
		}
		fmt.Println(string(jsonBytes))
	}
}

// countOperationTypes counts operations by type
func countOperationTypes(operations []map[string]interface{}) map[string]int {
	counts := map[string]int{
		"node":    0,
		"service": 0,
		"check":   0,
	}

	for _, op := range operations {
		if _, ok := op["Node"]; ok {
			counts["node"]++
		} else if _, ok := op["Service"]; ok {
			counts["service"]++
		} else if _, ok := op["Check"]; ok {
			counts["check"]++
		}
	}

	return counts
}

// printOperationsDetail prints detailed operation information
func printOperationsDetail(operations []map[string]interface{}) {
	fmt.Println("\n=== Operations Detail ===")

	maxDisplay := 10
	displayCount := min(len(operations), maxDisplay)

	for i := 0; i < displayCount; i++ {
		jsonBytes, err := json.MarshalIndent(operations[i], "", "  ")
		if err != nil {
			log.Printf("[WARN] Failed to marshal operation %d: %v", i, err)
			continue
		}
		fmt.Printf("\nOperation %d:\n%s\n", i+1, string(jsonBytes))
	}

	if len(operations) > maxDisplay {
		fmt.Printf("\n... and %d more operations\n", len(operations)-maxDisplay)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
