package tcp

import (
	"net"
	"strconv"
	"testing"
	"time"

	i2pconv "github.com/go-i2p/go-i2ptunnel-config/lib"
	client "github.com/go-i2p/go-i2ptunnel/lib/tcp/client"
	server "github.com/go-i2p/go-i2ptunnel/lib/tcp/server"
	"github.com/go-i2p/i2pkeys"
)

func RandomOpenPort(n string) int {
	switch n {
	case "tcp", "tcp4", "tcp6":
		l, _ := net.Listen(n, ":0")
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	case "udp", "udp4", "udp6":
		l, _ := net.ListenPacket(n, ":0")
		defer l.Close()
		return l.LocalAddr().(*net.UDPAddr).Port
	}
	return 0
}

func RandomTCPPort() int {
	return RandomOpenPort("tcp")
}

func RandomUDPPort() int {
	return RandomOpenPort("udp")
}

func TestTCPTunnel(t *testing.T) {
	t.Log("Starting TCP tunnel test")

	// Generate test keys
	t.Log("Loading test keys...")
	_, err := i2pkeys.LoadKeys("i2pkeys/test-server.i2p.private")
	if err != nil {
		t.Fatalf("Failed to load test keys: %v", err)
	}
	sport := RandomTCPPort()
	t.Logf("Selected server port: %d", sport)

	// Setup server config
	t.Log("Setting up server configuration...")
	serverConfig := i2pconv.TunnelConfig{
		Name:      "test-server",
		Type:      "tcpserver",
		Port:      sport,
		Interface: "127.0.0.1",
	}

	// Create and start server
	t.Log("Creating TCP server...")
	srv, err := server.NewTCPServer(serverConfig, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	t.Log("Starting TCP server...")
	go func() {
		if err := srv.Start(); err != nil {
			t.Errorf("Server error: %v", err)
		}
	}()
	defer srv.Stop()
	cport := RandomTCPPort()
	t.Logf("Selected client port: %d", cport)

	t.Log("Waiting for server startup...")
	time.Sleep(2 * time.Second)

	// Setup client config
	t.Log("Setting up client configuration...")
	clientConfig := i2pconv.TunnelConfig{
		Name:      "test-client",
		Type:      "tcpclient",
		Port:      cport,
		Interface: "127.0.0.1",
		Target:    srv.Address(),
	}

	// Create and start client
	t.Log("Creating TCP client...")
	cli, err := client.NewTCPClient(clientConfig, "127.0.0.1:7656")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Log("Starting TCP client...")
	go func() {
		if err := cli.Start(); err != nil {
			t.Errorf("Client error: %v", err)
		}
	}()
	defer cli.Stop()

	t.Log("Waiting for client startup...")
	time.Sleep(2 * time.Second)

	// Test data transfer
	testData := []byte("Hello I2P!")
	t.Log("Attempting to establish connection...")
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(cport)))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	t.Log("Connection established successfully")

	t.Logf("Writing test data: %q", testData)
	n, err := conn.Write(testData)
	if err != nil || n != len(testData) {
		t.Fatalf("Failed to write test data: %v", err)
	}
	t.Logf("Successfully wrote %d bytes", n)

	t.Log("Reading response...")
	buf := make([]byte, len(testData))
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}
	if n != len(testData) {
		t.Fatalf("Read wrong number of bytes: got %d, want %d", n, len(testData))
	}
	t.Logf("Successfully read %d bytes", n)

	if string(buf) != string(testData) {
		t.Errorf("Data mismatch: got %q, want %q", buf, testData)
	}
	t.Log("Test completed successfully")
}
