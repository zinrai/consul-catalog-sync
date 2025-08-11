package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// loadVars loads vars from a file or directory
func loadVars(path string, verbose bool) (map[string]interface{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access vars path: %w", err)
	}

	allVars := make(map[string]interface{})

	if !info.IsDir() {
		// Single file mode
		log.Printf("[INFO] Loading vars from file: %s", path)
		vars, err := loadYAMLFile(path)
		if err != nil {
			return nil, err
		}
		allVars = vars
		log.Printf("[INFO] Loaded %d nodes", len(allVars))
	} else {
		// Directory mode
		vars, err := loadVarsFromDirectory(path, verbose)
		if err != nil {
			return nil, err
		}
		allVars = vars
	}

	if len(allVars) == 0 {
		return nil, fmt.Errorf("no nodes found in %s", path)
	}

	return allVars, nil
}

// loadVarsFromDirectory loads all YAML files from a directory recursively
func loadVarsFromDirectory(path string, verbose bool) (map[string]interface{}, error) {
	log.Printf("[INFO] Loading vars from directory: %s", path)

	allVars := make(map[string]interface{})
	fileCount := 0

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-YAML files
		ext := filepath.Ext(p)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Process YAML file
		relPath, _ := filepath.Rel(path, p)
		if verbose {
			log.Printf("[DEBUG] Loading: %s", relPath)
		}

		vars, err := loadYAMLFile(p)
		if err != nil {
			log.Printf("[WARN] Failed to parse %s: %v", relPath, err)
			return nil // Skip this file but continue
		}

		fileCount++
		nodeCount := mergeVars(allVars, vars, relPath)
		log.Printf("[INFO] Loaded %d nodes from %s", nodeCount, relPath)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	log.Printf("[INFO] Total: %d files, %d nodes loaded", fileCount, len(allVars))
	return allVars, nil
}

// loadYAMLFile loads a single YAML file
func loadYAMLFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var result map[string]interface{}
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return result, nil
}

// loadMapping loads the mapping configuration file
func loadMapping(path string) (*MappingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read mapping file: %w", err)
	}

	var config MappingConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse mapping YAML: %w", err)
	}

	if len(config.Operations) == 0 {
		return nil, fmt.Errorf("no operations defined in mapping")
	}

	log.Printf("[INFO] Loaded mapping with %d operation rules", len(config.Operations))

	return &config, nil
}

// mergeVars merges source vars into target, checking for duplicates
func mergeVars(target, source map[string]interface{}, sourcePath string) int {
	added := 0
	for k, v := range source {
		if _, exists := target[k]; exists {
			log.Printf("[WARN] Duplicate node '%s' found in %s (keeping first occurrence)", k, sourcePath)
			continue
		}
		target[k] = v
		added++
	}
	return added
}
