// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNotifyIgnoresInvalidJSON(t *testing.T) {
	s, err := NewServer(&fakeHost{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.Notify([]byte("not json"))
	if got := s.events.snapshot(); len(got) != 0 {
		t.Fatalf("invalid event was buffered: %v", got)
	}
	s.Notify([]byte(`{"type":"document.created","document":"bracket"}`))
	if got := s.events.snapshot(); len(got) != 1 {
		t.Fatalf("valid event not buffered: %v", got)
	}
}

func TestEventBufferBounded(t *testing.T) {
	b := &eventBuffer{}
	for i := 0; i < maxRecentEvents+10; i++ {
		b.push([]byte(`{"type":"command.ended"}`))
	}
	if got := len(b.snapshot()); got != maxRecentEvents {
		t.Fatalf("buffer length = %d, want %d", got, maxRecentEvents)
	}
}

func TestRecentEventsResource(t *testing.T) {
	s, err := NewServer(&fakeHost{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.Notify([]byte(`{"type":"document.created","document":"bracket"}`))

	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := s.mcp.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			t.Fatalf("close client session: %v", closeErr)
		}
	}()

	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "oblikovati://events/recent"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) == 0 || !strings.Contains(res.Contents[0].Text, "bracket") {
		t.Fatalf("events/recent = %+v, want the buffered event", res.Contents)
	}
	var payload struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &payload); err != nil || len(payload.Events) != 1 {
		t.Fatalf("events payload = %s (err %v), want 1 event", res.Contents[0].Text, err)
	}
}
