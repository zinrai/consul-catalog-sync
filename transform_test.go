package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

// YAML to Consul API format transformation
func TestGenerateOperations(t *testing.T) {
	tests := []struct {
		name      string
		ctx       ExecutionContext
		mapping   *MappingConfig
		wantCount int
		wantFirst map[string]interface{}
	}{
		{
			name: "basic node and service operations",
			ctx: ExecutionContext{
				Key: "test-node-001",
				Value: map[string]interface{}{
					"field1": "service-a",
					"field2": "10.0.0.1",
					"field3": "type-a",
				},
				Datacenter: "dc1",
			},
			mapping: &MappingConfig{
				Operations: []OperationRule{
					{
						Type: "Node",
						Verb: "set",
						Template: map[string]interface{}{
							"Node":       "{{ .Key }}",
							"Address":    "{{ .Value.field2 }}",
							"Datacenter": "{{ .Datacenter }}",
							"Meta": map[string]interface{}{
								"type": "{{ .Value.field3 }}",
							},
						},
					},
					{
						Type:      "Service",
						Verb:      "set",
						Condition: "{{ .Value.field1 }}",
						Template: map[string]interface{}{
							"Node": "{{ .Key }}",
							"Service": map[string]interface{}{
								"ID":      "{{ .Value.field1 }}",
								"Service": "{{ .Value.field1 }}",
								"Tags": []interface{}{
									"{{ .Value.field3 }}",
								},
							},
						},
					},
				},
			},
			wantCount: 2,
			wantFirst: map[string]interface{}{
				"Node": map[string]interface{}{
					"Verb": "set",
					"Node": map[string]interface{}{
						"Node":       "test-node-001",
						"Address":    "10.0.0.1",
						"Datacenter": "dc1",
						"Meta": map[string]interface{}{
							"type": "type-a",
						},
					},
				},
			},
		},
		{
			name: "foreach processing with array",
			ctx: ExecutionContext{
				Key: "test-node-002",
				Value: map[string]interface{}{
					"field1": "main-service",
					"nested_field": []interface{}{
						map[string]interface{}{
							"item1": "sub1",
							"item2": float64(8080),
						},
						map[string]interface{}{
							"item1": "sub2",
							"item2": float64(9090),
						},
					},
				},
				Datacenter: "dc1",
			},
			mapping: &MappingConfig{
				Operations: []OperationRule{
					{
						Type:    "Service",
						Verb:    "set",
						Foreach: "{{ .Value.nested_field }}",
						Template: map[string]interface{}{
							"Node": "{{ .Key }}",
							"Service": map[string]interface{}{
								"ID":      "{{ .Item.item1 }}",
								"Service": "{{ .Value.field1 }}",
								"Port":    "{{ .Item.item2 }}",
							},
						},
					},
				},
			},
			wantCount: 2, // Two items in nested_field
		},
		{
			name: "condition false skips operation",
			ctx: ExecutionContext{
				Key: "test-node-003",
				Value: map[string]interface{}{
					"field2": "10.0.0.3",
					"field3": "type-c",
					// field1 is missing
				},
				Datacenter: "dc1",
			},
			mapping: &MappingConfig{
				Operations: []OperationRule{
					{
						Type:      "Service",
						Verb:      "set",
						Condition: "{{ .Value.field1 }}", // This will be empty
						Template: map[string]interface{}{
							"Node": "{{ .Key }}",
							"Service": map[string]interface{}{
								"ID":      "{{ .Value.field1 }}",
								"Service": "{{ .Value.field1 }}",
							},
						},
					},
				},
			},
			wantCount: 0, // No operations due to condition
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateOperations(tt.ctx, tt.mapping)
			if err != nil {
				t.Errorf("GenerateOperations() error = %v", err)
				return
			}

			if len(got) != tt.wantCount {
				t.Errorf("GenerateOperations() returned %d operations, want %d", len(got), tt.wantCount)
			}

			if tt.wantFirst != nil && len(got) > 0 {
				if !reflect.DeepEqual(got[0], tt.wantFirst) {
					gotJSON, _ := json.MarshalIndent(got[0], "", "  ")
					wantJSON, _ := json.MarshalIndent(tt.wantFirst, "", "  ")
					t.Errorf("First operation mismatch:\ngot:\n%s\nwant:\n%s", gotJSON, wantJSON)
				}
			}
		})
	}
}

// Template processing
func TestProcessTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template interface{}
		ctx      ExecutionContext
		want     interface{}
	}{
		{
			name:     "simple string template",
			template: "{{ .Key }}",
			ctx: ExecutionContext{
				Key: "node-001",
			},
			want: "node-001",
		},
		{
			name:     "nested field access",
			template: "{{ .Value.address }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{
					"address": "192.168.1.1",
				},
			},
			want: "192.168.1.1",
		},
		{
			name: "map template",
			template: map[string]interface{}{
				"name":    "{{ .Key }}",
				"address": "{{ .Value.addr }}",
				"meta": map[string]interface{}{
					"type": "{{ .Value.type }}",
				},
			},
			ctx: ExecutionContext{
				Key: "test-node",
				Value: map[string]interface{}{
					"addr": "10.0.0.1",
					"type": "web",
				},
			},
			want: map[string]interface{}{
				"name":    "test-node",
				"address": "10.0.0.1",
				"meta": map[string]interface{}{
					"type": "web",
				},
			},
		},
		{
			name: "array template",
			template: []interface{}{
				"{{ .Value.tag1 }}",
				"{{ .Value.tag2 }}",
			},
			ctx: ExecutionContext{
				Value: map[string]interface{}{
					"tag1": "web",
					"tag2": "production",
				},
			},
			want: []interface{}{"web", "production"},
		},
		{
			name:     "missing field returns empty",
			template: "{{ .Value.nonexistent }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{},
			},
			want: "",
		},
		{
			name:     "number conversion",
			template: "{{ .Value.port }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{
					"port": float64(8080),
				},
			},
			want: 8080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processTemplate(tt.template, tt.ctx)
			if err != nil {
				t.Errorf("processTemplate() error = %v", err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(tt.want)
				t.Errorf("processTemplate() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// Operation structure generation
func TestWrapOperations(t *testing.T) {
	tests := []struct {
		name   string
		opType string
		verb   string
		data   map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name:   "Node operation structure",
			opType: "Node",
			verb:   "set",
			data: map[string]interface{}{
				"Node":       "web-001",
				"Address":    "10.0.0.1",
				"Datacenter": "dc1",
				"Meta": map[string]interface{}{
					"role": "web",
				},
			},
			want: map[string]interface{}{
				"Node": map[string]interface{}{
					"Verb": "set",
					"Node": map[string]interface{}{
						"Node":       "web-001",
						"Address":    "10.0.0.1",
						"Datacenter": "dc1",
						"Meta": map[string]interface{}{
							"role": "web",
						},
					},
				},
			},
		},
		{
			name:   "Service operation structure",
			opType: "Service",
			verb:   "set",
			data: map[string]interface{}{
				"Node": "web-001",
				"Service": map[string]interface{}{
					"ID":      "nginx",
					"Service": "nginx",
					"Port":    80,
					"Tags":    []interface{}{"web", "primary"},
				},
			},
			want: map[string]interface{}{
				"Service": map[string]interface{}{
					"Verb": "set",
					"Node": "web-001",
					"Service": map[string]interface{}{
						"ID":      "nginx",
						"Service": "nginx",
						"Port":    80,
						"Tags":    []interface{}{"web", "primary"},
					},
				},
			},
		},
		{
			name:   "Check operation structure",
			opType: "Check",
			verb:   "set",
			data: map[string]interface{}{
				"Node": "web-001",
				"Check": map[string]interface{}{
					"CheckID": "service:nginx",
					"Name":    "HTTP Check",
					"Status":  "passing",
				},
			},
			want: map[string]interface{}{
				"Check": map[string]interface{}{
					"Verb": "set",
					"Node": "web-001",
					"Check": map[string]interface{}{
						"CheckID": "service:nginx",
						"Name":    "HTTP Check",
						"Status":  "passing",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wrapOperation(tt.opType, tt.verb, tt.data)
			if err != nil {
				t.Errorf("wrapOperation() error = %v", err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
				t.Errorf("wrapOperation() mismatch:\ngot:\n%s\nwant:\n%s", gotJSON, wantJSON)
			}
		})
	}
}

// Test Node operation wrapping specifically
func TestWrapNodeOperation(t *testing.T) {
	data := map[string]interface{}{
		"Node":       "test-node",
		"Address":    "10.0.0.1",
		"Datacenter": "dc1",
	}

	got := wrapNodeOperation("set", data)

	// Check structure is correct - Node.Verb and Node.Node
	nodeOp, ok := got["Node"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Node to be a map")
	}

	if nodeOp["Verb"] != "set" {
		t.Errorf("Expected Verb to be 'set', got %v", nodeOp["Verb"])
	}

	nodeData, ok := nodeOp["Node"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected Node.Node to be a map")
	}

	if nodeData["Address"] != "10.0.0.1" {
		t.Errorf("Expected Address to be '10.0.0.1', got %v", nodeData["Address"])
	}
}

// Test Service operation wrapping specifically
func TestWrapServiceOperation(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid service operation",
			data: map[string]interface{}{
				"Node": "web-001",
				"Service": map[string]interface{}{
					"ID":      "nginx",
					"Service": "nginx",
				},
			},
			wantErr: false,
		},
		{
			name: "missing Node field",
			data: map[string]interface{}{
				"Service": map[string]interface{}{
					"ID": "nginx",
				},
			},
			wantErr: true,
		},
		{
			name: "missing Service field",
			data: map[string]interface{}{
				"Node": "web-001",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wrapServiceOperation("set", tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("wrapServiceOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil {
				// Verify structure
				serviceOp, ok := got["Service"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected Service to be a map")
				}

				if serviceOp["Verb"] != "set" {
					t.Errorf("Expected Verb to be 'set', got %v", serviceOp["Verb"])
				}

				if serviceOp["Node"] != tt.data["Node"] {
					t.Errorf("Expected Node to be %v, got %v", tt.data["Node"], serviceOp["Node"])
				}
			}
		})
	}
}

// Test template evaluation
func TestEvaluateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		ctx      ExecutionContext
		want     string
		wantErr  bool
	}{
		{
			name:     "simple key reference",
			template: "{{ .Key }}",
			ctx:      ExecutionContext{Key: "node-001"},
			want:     "node-001",
		},
		{
			name:     "datacenter reference",
			template: "{{ .Datacenter }}",
			ctx:      ExecutionContext{Datacenter: "dc1"},
			want:     "dc1",
		},
		{
			name:     "nested value reference",
			template: "{{ .Value.service }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{
					"service": "nginx",
				},
			},
			want: "nginx",
		},
		{
			name:     "missing field returns empty",
			template: "{{ .Value.missing }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{},
			},
			want: "",
		},
		{
			name:     "item reference in foreach",
			template: "{{ .Item.name }}",
			ctx: ExecutionContext{
				Item: map[string]interface{}{
					"name": "ssh",
				},
			},
			want: "ssh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateTemplate(tt.template, tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("evaluateTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("evaluateTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test foreach evaluation
func TestEvaluateForeach(t *testing.T) {
	tests := []struct {
		name string
		expr string
		ctx  ExecutionContext
		want []interface{}
	}{
		{
			name: "evaluate nested_field array",
			expr: "{{ .Value.nested_field }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{
					"nested_field": []interface{}{
						map[string]interface{}{"item": "a"},
						map[string]interface{}{"item": "b"},
					},
				},
			},
			want: []interface{}{
				map[string]interface{}{"item": "a"},
				map[string]interface{}{"item": "b"},
			},
		},
		{
			name: "missing field returns nil",
			expr: "{{ .Value.missing }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{},
			},
			want: nil,
		},
		{
			name: "empty array",
			expr: "{{ .Value.empty }}",
			ctx: ExecutionContext{
				Value: map[string]interface{}{
					"empty": []interface{}{},
				},
			},
			want: []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateForeach(tt.expr, tt.ctx)
			if err != nil {
				t.Errorf("evaluateForeach() error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("evaluateForeach() = %v, want %v", got, tt.want)
			}
		})
	}
}
