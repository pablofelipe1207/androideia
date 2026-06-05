package mcpclient

import (
	"testing"
)

func TestNewMCPClient(t *testing.T) {
	// Test creating MCP client
	serverURL := "http://localhost:3000"
	client := NewMCPClient(serverURL)
	
	if client == nil {
		t.Fatal("MCP client is nil")
	}
	
	if client.serverURL != serverURL {
		t.Errorf("Expected server URL '%s', got '%s'", serverURL, client.serverURL)
	}
	
	if client.timeout != 30000000000 { // 30 seconds in nanoseconds
		t.Errorf("Expected timeout 30000000000, got %d", client.timeout)
	}
}

func TestIsConnected(t *testing.T) {
	// Test that client is not connected initially
	client := NewMCPClient("http://localhost:3000")
	
	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

func TestGetServerInfo(t *testing.T) {
	// Test getting server info
	serverURL := "http://localhost:3000"
	client := NewMCPClient(serverURL)
	
	info := client.GetServerInfo()
	
	if info["url"] != serverURL {
		t.Errorf("Expected URL '%s', got '%v'", serverURL, info["url"])
	}
	
	if info["connected"] != false {
		t.Error("Expected connected to be false")
	}
}

func TestDisconnectWithoutConnection(t *testing.T) {
	// Test disconnecting without connection
	client := NewMCPClient("http://localhost:3000")
	
	// This should not panic
	err := client.Disconnect(nil)
	if err != nil {
		t.Errorf("Error disconnecting: %v", err)
	}
}

func TestListToolsWithoutConnection(t *testing.T) {
	// Test listing tools without connection
	client := NewMCPClient("http://localhost:3000")
	
	_, err := client.ListTools(nil)
	if err == nil {
		t.Error("Expected error when listing tools without connection")
	}
}

func TestCallToolWithoutConnection(t *testing.T) {
	// Test calling tool without connection
	client := NewMCPClient("http://localhost:3000")
	
	_, err := client.CallTool(nil, "test-tool", nil)
	if err == nil {
		t.Error("Expected error when calling tool without connection")
	}
}

func TestListResourcesWithoutConnection(t *testing.T) {
	// Test listing resources without connection
	client := NewMCPClient("http://localhost:3000")
	
	_, err := client.ListResources(nil)
	if err == nil {
		t.Error("Expected error when listing resources without connection")
	}
}

func TestReadResourceWithoutConnection(t *testing.T) {
	// Test reading resource without connection
	client := NewMCPClient("http://localhost:3000")
	
	_, err := client.ReadResource(nil, "test://resource")
	if err == nil {
		t.Error("Expected error when reading resource without connection")
	}
}
