package mcp

import (
	"encoding/json"
	"testing"

	"workbench/internal/catalog"
)

func TestToolSchemasExplicitlyRejectAdditionalProperties(t *testing.T) {
	server := New(catalog.Store{Root: t.TempDir()}, true)

	for _, definition := range server.tools() {
		t.Run(definition.Name, func(t *testing.T) {
			encoded, err := json.Marshal(definition.InputSchema)
			if err != nil {
				t.Fatal(err)
			}

			var schema map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &schema); err != nil {
				t.Fatal(err)
			}
			if string(schema["additionalProperties"]) != "false" {
				t.Fatal("typed schema dropped the explicit unknown-field restriction", string(encoded))
			}

			if definition.Name == "register_project" {
				var properties map[string]map[string]json.RawMessage
				if err := json.Unmarshal(schema["properties"], &properties); err != nil {
					t.Fatal(err)
				}
				if string(properties["project"]["additionalProperties"]) != "false" {
					t.Fatal("nested project schema allows unknown fields", string(encoded))
				}
			}
		})
	}
}
