// SPDX-License-Identifier: GPL-2.0-only
package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

func TestCameraToolsForwardDocumentAndProjection(t *testing.T) {
	host := &fakeHost{reply: []byte(`{}`)}
	client := connect(t, host)
	for _, tc := range []struct {
		name       string
		method     string
		args       map[string]any
		projection types.ProjectionTypeEnum
	}{
		{"get_camera", wire.MethodViewGetCamera, map[string]any{"document": 8}, 0},
		{"set_camera", wire.MethodViewSetCamera, map[string]any{
			"document": 8,
			"eye":      []float64{2, 3, 4}, "target": []float64{0, 0, 0},
			"up": []float64{0, 1, 0}, "fov": 0.6,
			"projection": types.PerspectiveWithOrthoFacesProjection,
		}, types.PerspectiveWithOrthoFacesProjection},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reply, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil || reply.IsError {
				t.Fatalf("camera tool rejected request: reply=%v error=%v", reply, err)
			}
			var forwarded struct {
				Document   uint64                   `json:"document"`
				Projection types.ProjectionTypeEnum `json:"projection"`
			}
			if err := json.Unmarshal(host.lastReq, &forwarded); err != nil {
				t.Fatal(err)
			}
			if host.lastMethod != tc.method || forwarded.Document != 8 || forwarded.Projection != tc.projection {
				t.Fatalf("forwarded %s %s, want %s for document 8 with projection %d", host.lastMethod, host.lastReq, tc.method, tc.projection)
			}
		})
	}
}

func TestGetCameraWithoutArgumentsUsesActiveDocument(t *testing.T) {
	host := &fakeHost{reply: []byte(`{}`)}
	reply, err := connect(t, host).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_camera"})
	if err != nil || reply.IsError || host.lastMethod != wire.MethodViewGetCamera || string(host.lastReq) != "{}" {
		t.Fatalf("default camera request %s %s: reply=%v error=%v", host.lastMethod, host.lastReq, reply, err)
	}
}
