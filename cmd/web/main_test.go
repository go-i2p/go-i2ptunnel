package main

import (
	"flag"
	"os"
	"os/exec"
	"testing"
)

// TestWebMissingConfigFlag verifies the web UI binary exits non-zero for an invalid port.
// cmd/web uses panic (not os.Exit) on errors, so a bad port triggers a non-zero exit.
func TestWebInvalidPort(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		// port 99999 is out of valid range; net.Listen will fail and main() will panic
		os.Args = []string{"prog", "-port", "99999"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestWebInvalidPort")
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit for invalid port, got success")
	}
}
