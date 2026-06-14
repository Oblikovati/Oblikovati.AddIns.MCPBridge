// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxRecentEvents bounds the in-memory event buffer (newest kept).
const maxRecentEvents = 100

// eventBuffer holds the most recent host events the add-in has been notified of, so
// the LLM can read them as a resource. MCP has no generic server->client event for
// arbitrary domain events, so we expose a pollable resource (oblikovati://events/
// recent); the buffer is filled by Notify over the C ABI.
type eventBuffer struct {
	mu     sync.Mutex
	recent []json.RawMessage
}

// push appends an event, dropping the oldest beyond maxRecentEvents.
func (b *eventBuffer) push(ev []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recent = append(b.recent, append(json.RawMessage(nil), ev...))
	if len(b.recent) > maxRecentEvents {
		b.recent = b.recent[len(b.recent)-maxRecentEvents:]
	}
}

// snapshot returns a copy of the buffered events, oldest first.
func (b *eventBuffer) snapshot() []json.RawMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]json.RawMessage, len(b.recent))
	copy(out, b.recent)
	return out
}

// Notify records a host event (called from the C-ABI ObkAddInNotify). A malformed
// (non-JSON) event is ignored so the host can't corrupt the resource.
func (s *Server) Notify(ev []byte) {
	if !json.Valid(ev) {
		return
	}
	s.events.push(ev)
}

// registerEventsResource serves the recent host events as a JSON array.
func (s *Server) registerEventsResource() {
	const uri = "oblikovati://events/recent"
	s.mcp.AddResource(
		&mcp.Resource{URI: uri, Name: "Recent events", Description: "Recent host events (documents created/saved/activated, commands finished).", MIMEType: "application/json"},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			body, err := json.Marshal(struct {
				Events []json.RawMessage `json:"events"`
			}{s.events.snapshot()})
			if err != nil {
				return nil, err
			}
			return textResource(uri, "application/json", string(body)), nil
		})
}
