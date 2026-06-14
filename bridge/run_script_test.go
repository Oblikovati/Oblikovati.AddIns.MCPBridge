// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

// TestRunScriptForwardsToHost: the run_script tool forwards the Lua source (and an optional
// wallMs) to the host's scripts.run method and returns its reply.
func TestRunScriptForwardsToHost(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"output":"hi\n","durationMs":4}`)}
	cs := connect(t, host)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "run_script",
		Arguments: map[string]any{"source": `print("hi")`, "wallMs": 5000},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	if host.lastMethod != wire.MethodScriptRun {
		t.Fatalf("forwarded method = %q, want %q", host.lastMethod, wire.MethodScriptRun)
	}
	var sent wire.ScriptRunArgs
	if err := json.Unmarshal(host.lastReq, &sent); err != nil {
		t.Fatalf("forwarded req not JSON: %v", err)
	}
	if sent.Source != `print("hi")` || sent.WallMs != 5000 {
		t.Fatalf("forwarded args = %+v, want source+wallMs preserved", sent)
	}
}
