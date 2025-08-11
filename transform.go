package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"
)

// MappingConfig represents the mapping configuration
type MappingConfig struct {
	Operations []OperationRule `yaml:"operations"`
}

// OperationRule defines how to transform vars data into Consul operations
type OperationRule struct {
	Type      string                 `yaml:"type"`      // Node, Service, Check
	Verb      string                 `yaml:"verb"`      // set, delete, cas
	Condition string                 `yaml:"condition"` // Template condition for execution
	Foreach   string                 `yaml:"foreach"`   // Template for iteration
	Template  map[string]interface{} `yaml:"template"`  // Operation template
}

// ExecutionContext holds the context for template execution
type ExecutionContext struct {
	Key        string                 // Node name from vars
	Value      map[string]interface{} // Node data from vars
	Datacenter string                 // From command line
	Item       interface{}            // Current item in foreach loop
}

// GenerateOperations transforms a single node using mapping rules
func GenerateOperations(ctx ExecutionContext, config *MappingConfig) ([]map[string]interface{}, error) {
	var operations []map[string]interface{}

	for _, rule := range config.Operations {
		// Check condition
		if rule.Condition != "" {
			result, err := evaluateTemplate(rule.Condition, ctx)
			if err != nil {
				log.Printf("[WARN] Failed to evaluate condition for %s: %v", ctx.Key, err)
				continue
			}
			// Skip if condition evaluates to empty or "false"
			if result == "" || result == "false" || result == "<no value>" {
				continue
			}
		}

		// Handle foreach
		if rule.Foreach != "" {
			foreachOps, err := processForeach(rule, ctx)
			if err != nil {
				log.Printf("[WARN] Failed to process foreach for %s: %v", ctx.Key, err)
				continue
			}
			operations = append(operations, foreachOps...)
		} else {
			// Single operation
			op, err := generateSingleOperation(rule, ctx)
			if err != nil {
				log.Printf("[WARN] Failed to generate operation for %s: %v", ctx.Key, err)
				continue
			}
			if op != nil {
				operations = append(operations, op)
			}
		}
	}

	return operations, nil
}

func generateSingleOperation(rule OperationRule, ctx ExecutionContext) (map[string]interface{}, error) {
	// Process template
	processed, err := processTemplate(rule.Template, ctx)
	if err != nil {
		return nil, fmt.Errorf("template processing failed: %w", err)
	}

	processedMap, ok := processed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("template result is not a map")
	}

	// Set default verb if not specified
	verb := rule.Verb
	if verb == "" {
		verb = "set"
	}

	// Wrap in Consul API format based on type
	return wrapOperation(rule.Type, verb, processedMap)
}

func wrapOperation(opType, verb string, data map[string]interface{}) (map[string]interface{}, error) {
	switch opType {
	case "Node":
		return wrapNodeOperation(verb, data), nil

	case "Service":
		return wrapServiceOperation(verb, data)

	case "Check":
		return wrapCheckOperation(verb, data)

	default:
		return nil, fmt.Errorf("unknown operation type: %s", opType)
	}
}

func wrapNodeOperation(verb string, data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"Node": map[string]interface{}{
			"Verb": verb,
			"Node": data, // dataを"Node"フィールドの値として正しくネスト
		},
	}
}

func wrapServiceOperation(verb string, data map[string]interface{}) (map[string]interface{}, error) {
	// Extract node name and service data
	nodeName, _ := data["Node"].(string)
	serviceData, _ := data["Service"].(map[string]interface{})

	if nodeName == "" || serviceData == nil {
		return nil, fmt.Errorf("invalid service operation: missing Node or Service")
	}

	return map[string]interface{}{
		"Service": map[string]interface{}{
			"Verb":    verb,
			"Node":    nodeName,
			"Service": serviceData,
		},
	}, nil
}

func wrapCheckOperation(verb string, data map[string]interface{}) (map[string]interface{}, error) {
	// Extract node name and check data
	nodeName, _ := data["Node"].(string)
	checkData, _ := data["Check"].(map[string]interface{})

	if nodeName == "" || checkData == nil {
		return nil, fmt.Errorf("invalid check operation: missing Node or Check")
	}

	return map[string]interface{}{
		"Check": map[string]interface{}{
			"Verb":  verb,
			"Node":  nodeName,
			"Check": checkData,
		},
	}, nil
}

func processForeach(rule OperationRule, ctx ExecutionContext) ([]map[string]interface{}, error) {
	// Evaluate foreach expression to get items
	items, err := evaluateForeach(rule.Foreach, ctx)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, nil
	}

	var operations []map[string]interface{}

	for _, item := range items {
		// Create context with Item
		itemCtx := ExecutionContext{
			Key:        ctx.Key,
			Value:      ctx.Value,
			Datacenter: ctx.Datacenter,
			Item:       item,
		}

		op, err := generateSingleOperation(rule, itemCtx)
		if err != nil {
			log.Printf("[WARN] Failed to generate operation for item in %s: %v", ctx.Key, err)
			continue
		}
		if op != nil {
			operations = append(operations, op)
		}
	}

	return operations, nil
}

func evaluateForeach(expr string, ctx ExecutionContext) ([]interface{}, error) {
	// Use Go template to evaluate the expression
	tmpl, err := template.New("foreach").Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid foreach expression: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		// The field might not exist, which is ok
		return nil, nil
	}

	result := buf.String()
	if result == "" || result == "<no value>" {
		return nil, nil
	}

	// The template should have returned a reference to an array
	// We need to actually get the array from the context
	// This is a simplified approach - in production you might want
	// to use a more sophisticated expression evaluator

	// Try to get the array directly from the Value map
	// Parse the expression to extract the field path
	fieldPath := parseFieldPath(expr)
	if fieldPath != "" {
		if arr := getNestedField(ctx.Value, fieldPath); arr != nil {
			if items, ok := arr.([]interface{}); ok {
				return items, nil
			}
		}
	}

	// Fallback: try to parse as JSON
	var items []interface{}
	if err := json.Unmarshal([]byte(result), &items); err == nil {
		return items, nil
	}

	return nil, nil
}

func parseFieldPath(expr string) string {
	// Extract field path from template expression
	// {{ .Value.fieldname }} -> fieldname
	// This is a simple regex-based approach

	// Remove template delimiters and whitespace
	expr = strings.TrimSpace(expr)
	expr = strings.TrimPrefix(expr, "{{")
	expr = strings.TrimSuffix(expr, "}}")
	expr = strings.TrimSpace(expr)

	// Check if it matches .Value.something pattern
	if strings.HasPrefix(expr, ".Value.") {
		return strings.TrimPrefix(expr, ".Value.")
	}

	return ""
}

func getNestedField(data map[string]interface{}, path string) interface{} {
	// Support simple field access (no deep nesting for now)
	// "field1" -> data["field1"]
	// Could be extended to support "field1.field2" in the future

	if value, ok := data[path]; ok {
		return value
	}

	return nil
}

func processTemplate(templateData interface{}, ctx ExecutionContext) (interface{}, error) {
	switch v := templateData.(type) {
	case string:
		// Process string template
		result, err := evaluateTemplate(v, ctx)
		if err != nil {
			return nil, err
		}
		// Try to parse as number if possible
		var num float64
		if err := json.Unmarshal([]byte(result), &num); err == nil {
			return int(num), nil
		}
		return result, nil

	case map[string]interface{}:
		// Process map recursively
		result := make(map[string]interface{})
		for key, value := range v {
			processed, err := processTemplate(value, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to process key %s: %w", key, err)
			}
			// Skip nil values
			if processed != nil && processed != "" && processed != "<no value>" {
				result[key] = processed
			}
		}
		return result, nil

	case []interface{}:
		// Process array
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			processed, err := processTemplate(item, ctx)
			if err != nil {
				return nil, err
			}
			// Skip nil values
			if processed != nil && processed != "" && processed != "<no value>" {
				result = append(result, processed)
			}
		}
		return result, nil

	default:
		// Return as-is for other types
		return v, nil
	}
}

func evaluateTemplate(templateStr string, ctx ExecutionContext) (string, error) {
	// Create custom functions
	funcMap := template.FuncMap{
		"default": func(defaultVal, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultVal
			}
			return value
		},
		"toJSON": func(v interface{}) (string, error) {
			bytes, err := json.Marshal(v)
			return string(bytes), err
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	result := buf.String()

	// Handle empty results
	if result == "<no value>" || result == "" {
		return "", nil
	}

	return result, nil
}
