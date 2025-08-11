package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	maxOperationsPerTransaction = 64
	defaultTimeout              = 30 * time.Second
)

// ExecuteOperations sends operations to Consul Transaction API
func ExecuteOperations(consulAddr string, operations []map[string]interface{}, verbose bool) error {
	if len(operations) == 0 {
		log.Printf("[WARN] No operations to execute")
		return nil
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: defaultTimeout,
	}

	// Process in batches
	totalBatches := (len(operations) + maxOperationsPerTransaction - 1) / maxOperationsPerTransaction

	for i := 0; i < len(operations); i += maxOperationsPerTransaction {
		end := min(i+maxOperationsPerTransaction, len(operations))
		batch := operations[i:end]

		batchNum := (i / maxOperationsPerTransaction) + 1
		log.Printf("[INFO] Executing batch %d/%d (%d operations)", batchNum, totalBatches, len(batch))

		if verbose {
			log.Printf("[DEBUG] Batch %d contains %d operations", batchNum, len(batch))
		}

		err := executeTransaction(client, consulAddr, batch, verbose)
		if err != nil {
			return fmt.Errorf("batch %d failed: %w", batchNum, err)
		}

		log.Printf("[OK] Batch %d/%d completed successfully", batchNum, totalBatches)
	}

	return nil
}

func executeTransaction(client *http.Client, consulAddr string, operations []map[string]interface{}, verbose bool) error {
	// Prepare the transaction payload
	payload, err := json.Marshal(operations)
	if err != nil {
		return fmt.Errorf("failed to marshal operations: %w", err)
	}

	if verbose {
		logVerboseInfo(operations, payload)
	}

	// Create and execute request
	resp, err := sendRequest(client, consulAddr, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Process response
	return processResponse(resp, verbose)
}

func logVerboseInfo(operations []map[string]interface{}, payload []byte) {
	log.Printf("[DEBUG] Request payload size: %d bytes", len(payload))

	// Log first operation as sample
	if len(operations) == 0 {
		return
	}

	firstOp, err := json.MarshalIndent(operations[0], "", "  ")
	if err != nil {
		return
	}

	log.Printf("[DEBUG] First operation in batch:\n%s", string(firstOp))
}

func sendRequest(client *http.Client, consulAddr string, payload []byte) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1/txn", consulAddr)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

func processResponse(resp *http.Response, verbose bool) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Success case
	if resp.StatusCode == http.StatusOK {
		if verbose {
			logSuccessDetails(resp.StatusCode, body)
		}
		return nil
	}

	// Error cases
	if resp.StatusCode == http.StatusConflict {
		return handleTransactionConflict(body, resp.StatusCode)
	}

	// Other HTTP errors
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

func logSuccessDetails(statusCode int, body []byte) {
	log.Printf("[DEBUG] Transaction successful, status: %d", statusCode)

	var result TransactionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	log.Printf("[DEBUG] Transaction results: %d successful operations", len(result.Results))
}

func handleTransactionConflict(body []byte, statusCode int) error {
	var result TransactionResponse
	if err := json.Unmarshal(body, &result); err == nil {
		return formatTransactionErrors(result.Errors)
	}

	return fmt.Errorf("transaction rolled back (status %d): %s", statusCode, string(body))
}

// TransactionResponse represents the response from Consul Transaction API
type TransactionResponse struct {
	Results []map[string]interface{} `json:"Results"`
	Errors  []TransactionError       `json:"Errors"`
}

// TransactionError represents an error in transaction response
type TransactionError struct {
	OpIndex int    `json:"OpIndex"`
	What    string `json:"What"`
}

func formatTransactionErrors(errors []TransactionError) error {
	if len(errors) == 0 {
		return fmt.Errorf("transaction failed with unknown error")
	}

	// Log each error
	for _, err := range errors {
		log.Printf("[ERROR] Operation %d failed: %s", err.OpIndex, err.What)
	}

	// Return first error as main error
	return fmt.Errorf("transaction failed: operation %d: %s", errors[0].OpIndex, errors[0].What)
}
