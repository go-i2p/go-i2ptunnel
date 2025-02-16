package tcp

import (
	"io"
	"net"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	client "github.com/go-i2p/go-i2ptunnel/lib/tcp/client"
	server "github.com/go-i2p/go-i2ptunnel/lib/tcp/server"
	"github.com/go-i2p/i2pkeys"
)

func TestTCPTunnel(t *testing.T) {
	// Generate test keys
	//keys, err := i2pkeys.LoadKeys("i2pkeys/test-server.i2p.private")
	_, err := i2pkeys.LoadKeys("i2pkeys/test-server.i2p.private")
	if err != nil {
		t.Fatalf("Failed to load test keys: %v", err)
	}

	// Setup server config
	serverConfig := i2pconv.TunnelConfig{
		Name:      "test-server",
		Type:      "tcpserver",
		Port:      8080,
		Interface: "127.0.0.1",
	}

	// Create and start server
	srv, err := server.NewTCPServer(serverConfig, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		if err := srv.Start(); err != nil {
			t.Errorf("Server error: %v", err)
		}
	}()
	defer srv.Stop()

	// Wait for server startup
	time.Sleep(2 * time.Second)

	// Setup client config
	clientConfig := i2pconv.TunnelConfig{
		Name:      "test-client",
		Type:      "tcpclient",
		Port:      8081,
		Interface: "127.0.0.1",
		Target:    srv.Address(),
	}

	// Create and start client
	cli, err := client.NewTCPClient(clientConfig, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	go func() {
		if err := cli.Start(); err != nil {
			t.Errorf("Client error: %v", err)
		}
	}()
	defer cli.Stop()

	// Wait for client startup
	time.Sleep(2 * time.Second)

	// Test data transfer
	testData := []byte("Hello I2P!")
	conn, err := net.Dial("tcp", "127.0.0.1:8081")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	n, err := conn.Write(testData)
	if err != nil || n != len(testData) {
		t.Fatalf("Failed to write test data: %v", err)
	}

	buf := make([]byte, len(testData))
	n, err = io.ReadFull(conn, buf)
	if err != nil || n != len(testData) {
		t.Fatalf("Failed to read test data: %v", err)
	}

	if string(buf) != string(testData) {
		t.Errorf("Data mismatch: got %q, want %q", buf, testData)
	}
}
