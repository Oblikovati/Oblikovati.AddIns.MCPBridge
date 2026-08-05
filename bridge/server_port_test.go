// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Two Oblikovati instances cannot share the MCP port. The second must fail LOUDLY, naming the
// address, because the failure mode is otherwise invisible and misleading: the add-in returned a
// bare error code, the host logged only "returned 1", and a client that connected then drove the
// FIRST instance's model. It looked like a registration race — a tool this build has answering
// "unknown tool" while every older tool worked — when it was really a different, older build on
// the other end of the socket (#2035).

// TestSecondServerRefusesABusyPort pins the refusal and the message that makes it diagnosable.
func TestSecondServerRefusesABusyPort(t *testing.T) {
	first, err := NewServer(&fakeHost{})
	if err != nil {
		t.Fatalf("first server: %v", err)
	}
	if err := first.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer func() { _ = first.Stop() }()

	second, err := NewServer(&fakeHost{})
	if err != nil {
		t.Fatalf("second server: %v", err)
	}

	err = second.Start(first.Addr())

	if err == nil {
		_ = second.Stop()
		t.Fatal("the second server took a port already in use — a client cannot tell the two apart")
	}
	if !strings.Contains(err.Error(), first.Addr()) {
		t.Errorf("error %q should name the address that was busy", err)
	}
}

// TestToolCountReportsTheRegisteredSurface: the activation line logs how many tools this server
// answers with, so an operator can see WHICH build is on the port when two are in play.
func TestToolCountReportsTheRegisteredSurface(t *testing.T) {
	s, err := NewServer(&fakeHost{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if s.ToolCount() < 100 {
		t.Errorf("ToolCount = %d, want the full generated tool surface", s.ToolCount())
	}
}

// TestToolCountMatchesWhatIsServed keeps the reported number honest as registrars are added: it
// compares the count against the tools a real client session lists. A registrar that forgets to
// count would otherwise make the activation line understate the surface — the one number an
// operator uses to tell two builds apart.
func TestToolCountMatchesWhatIsServed(t *testing.T) {
	host := &fakeHost{reply: []byte("{}")}
	s, cs := connectTo(t, host) // ONE server: counting a different instance proves nothing
	listed, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(listed.Tools) != s.ToolCount() {
		t.Errorf("ToolCount = %d but %d tools are served — a registrar is not counting itself",
			s.ToolCount(), len(listed.Tools))
	}
}

// connectTo builds one server and an in-memory client session against THAT server, so a test can
// compare the server's own bookkeeping with what it serves.
func connectTo(t *testing.T, host HostCaller) (*Server, *mcp.ClientSession) {
	t.Helper()
	s, err := NewServer(host)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx := t.Context()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := s.mcp.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return s, cs
}
