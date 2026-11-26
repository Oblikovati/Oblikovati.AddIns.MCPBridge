// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

func TestToolForwardsTypedPropertyValues(t *testing.T) {
	host := &fakeHost{reply: []byte("{}")}
	client := connect(t, host)
	for _, value := range []string{
		`{"type":"string","value":"PN-42"}`,
		`{"type":"integer","value":9223372036854775807}`,
		`{"type":"double","value":2.5,"unit":"mm"}`,
		`{"type":"boolean","value":false}`,
		`{"type":"bytes","value":"AP8="}`,
		`{"type":"bytes","value":null}`,
	} {
		t.Run(value, func(t *testing.T) {
			callJSON(t, client, "documents_set_property", map[string]any{
				"document": 1, "set": "Design Tracking Properties", "name": "Part Number",
				"value": json.RawMessage(value),
			}, nil)
			var request wire.SetPropertyArgs
			if err := json.Unmarshal(host.lastReq, &request); err != nil {
				t.Fatalf("decode host request: %v", err)
			}
			got, err := json.Marshal(request.Value)
			if err != nil || string(got) != value || host.lastMethod != wire.MethodDocumentsSetProperty {
				t.Fatalf("forwarded %s %s (%v), want %s", host.lastMethod, got, err, value)
			}
		})
	}
}

func TestToolRejectsMismatchedVariantPayloadBeforeCallingHost(t *testing.T) {
	host := &fakeHost{reply: []byte("{}")}
	client := connect(t, host)
	_, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "documents_set_property",
		Arguments: map[string]any{
			"document": 1, "set": "Design Tracking Properties", "name": "Part Number",
			"value": map[string]any{"type": "boolean", "value": "false"},
		},
	})
	if err == nil || host.lastMethod != "" {
		t.Fatalf("unexpected invalid payload outcome: error=%v, host method=%q", err, host.lastMethod)
	}
}

func TestToolForwardsInheritedAndExactModelStatePropertyValues(t *testing.T) {
	host := &fakeHost{reply: []byte("{}")}
	client := connect(t, host)
	callJSON(t, client, "set_model_state_table", map[string]any{
		"expectedGeneration": json.Number("18446744073709551615"),
		"rows":               nil, "columns": nil, "suppressions": nil, "parameters": nil,
		"properties": []any{map[string]any{
			"property": map[string]any{"set": "Design Tracking Properties", "name": "Part Number"},
			"base":     nil,
			"cells": []any{map[string]any{
				"row":   map[string]any{"memberId": 1},
				"value": json.RawMessage(`{"type":"integer","value":-9223372036854775808}`),
			}},
		}},
	}, nil)
	var got wire.SetModelStateTableArgs
	if err := json.Unmarshal(host.lastReq, &got); err != nil {
		t.Fatalf("decode model-state request: %v", err)
	}
	if got.ExpectedGeneration != ^uint64(0) || len(got.Properties) != 1 ||
		got.Properties[0].Base != nil || len(got.Properties[0].Cells) != 1 {
		t.Fatalf("forwarded wrong model-state draft: %s", host.lastReq)
	}
	cell := got.Properties[0].Cells[0].Value
	if cell == nil {
		t.Fatal("missing authored property")
	}
	if n, ok := cell.Integer(); !ok || n != -9223372036854775808 {
		t.Fatalf("forwarded integer = %d (valid: %t), want minimum int64", n, ok)
	}
}
