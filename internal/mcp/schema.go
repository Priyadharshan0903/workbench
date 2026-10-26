package mcp

// jsonSchema models the JSON Schema keywords used by Workbench's tool inputs.
// Property names vary by tool; each property's definition has a concrete shape.
type jsonSchema struct {
	Type                 string                `json:"type"`
	Properties           map[string]jsonSchema `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	AdditionalProperties *bool                 `json:"additionalProperties,omitempty"`
	Pattern              string                `json:"pattern,omitempty"`
	Minimum              int                   `json:"minimum,omitempty"`
	Maximum              int                   `json:"maximum,omitempty"`
	Enum                 []string              `json:"enum,omitempty"`
	Items                *jsonSchema           `json:"items,omitempty"`
	UniqueItems          bool                  `json:"uniqueItems,omitempty"`
	MaxItems             int                   `json:"maxItems,omitempty"`
}

func objectSchema(properties map[string]jsonSchema, required ...string) jsonSchema {
	allowAdditionalProperties := false

	return jsonSchema{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: &allowAdditionalProperties,
	}
}
