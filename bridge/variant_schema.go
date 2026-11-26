// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"

	"oblikovati.org/api/types"
)

// Variant's custom JSON representation is invisible to reflection of its private fields.
func toolInputSchema[Input any]() *jsonschema.Schema {
	schema, err := jsonschema.For[Input](&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[types.Variant](): variantSchema(),
		},
	})
	if err != nil {
		// The existing registration fallback handles types the SDK cannot describe.
		panic(err)
	}
	return schema
}

func variantSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"type":  {Type: "string"},
			"value": {},
			"unit":  {Type: "string"},
		},
		Required:             []string{"type", "value"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		AnyOf: []*jsonschema.Schema{
			variantCase("integer", &jsonschema.Schema{Type: "integer"}),
			variantCase("double", &jsonschema.Schema{Type: "number"}),
			variantCase("string", &jsonschema.Schema{Type: "string"}),
			variantCase("boolean", &jsonschema.Schema{Type: "boolean"}),
			variantCase("bytes", &jsonschema.Schema{Types: []string{"string", "null"}, ContentEncoding: "base64"}),
		},
	}
}

func variantCase(tag string, payload *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{
		Properties: map[string]*jsonschema.Schema{
			"type":  {Enum: []any{tag}},
			"value": payload,
		},
	}
}
