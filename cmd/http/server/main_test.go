package main

import (
	"flag"
	"os"
	"os/exec"
	"testing"
)

// TestRunMissingConfigFlag verifies the binary exits non-zero when -config is omitted.
func TestRunMissingConfigFlag(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		os.Args = []string{"prog"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestRunMissingConfigFlag")
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit when -config is missing, got success")
	}
}

// TestRunMissingConfigFile verifies the binary exits non-zero when the config file does not exist.
func TestRunMissingConfigFile(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		os.Args = []string{"prog", "-config", "/nonexistent/path/tunnel.yaml"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestRunMissingConfigFile")
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit for missing config file, got success")
	}
}
