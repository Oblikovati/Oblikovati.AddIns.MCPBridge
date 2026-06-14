// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

// TestRedefineWorkPlaneForwardsFullPayload guards the jsonschema/typed-args layer end to
// end: a redefine carrying an index, a scalar edit AND a slot repick must reach the host
// with all three intact (TestRedefineWorkPlaneForwardsArgs pins only the index). The values
// are deliberately non-zero so a field silently dropped to its zero value fails the test.
func TestRedefineWorkPlaneForwardsFullPayload(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"plane":{"index":2,"name":"Work Plane1","healthy":true}}`)}
	cs := connect(t, host)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "redefine_work_plane",
		Arguments: map[string]any{
			"index":   2,
			"scalars": []map[string]any{{"index": 1, "value": "45 deg"}},
			"repick":  []map[string]any{{"slot": 1, "ref": "origin/plane/xz"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	if host.lastMethod != wire.MethodWorkPlanesRedefine {
		t.Fatalf("forwarded method = %q, want %q", host.lastMethod, wire.MethodWorkPlanesRedefine)
	}
	var got wire.RedefineWorkPlaneArgs
	if err := json.Unmarshal(host.lastReq, &got); err != nil {
		t.Fatalf("forwarded req %s does not unmarshal as RedefineWorkPlaneArgs: %v", host.lastReq, err)
	}
	if got.Index != 2 {
		t.Errorf("forwarded index = %d, want 2", got.Index)
	}
	if len(got.Scalars) != 1 || got.Scalars[0].Index != 1 || got.Scalars[0].Value != "45 deg" {
		t.Errorf("forwarded scalars = %+v, want [{Index:1 Value:\"45 deg\"}]", got.Scalars)
	}
	if len(got.Repick) != 1 || got.Repick[0].Slot != 1 || got.Repick[0].Ref != "origin/plane/xz" {
		t.Errorf("forwarded repick = %+v, want [{Slot:1 Ref:\"origin/plane/xz\"}]", got.Repick)
	}
}
