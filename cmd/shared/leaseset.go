package shared

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	i2ptunnel "github.com/go-i2p/go-i2ptunnel/lib/core"
)

// promptLeaseSetCredential checks whether the tunnel requires an encrypted
// LeaseSet credential (leaseSetAuthType 1 = DH or 2 = PSK) and, if so,
// reads the Base64-encoded private key from r and injects it via SetOptions.
// Output (prompts) is written to w.
//
// When leaseSetAuthType is 0 (or unset), this function returns nil immediately
// without reading from r.
func promptLeaseSetCredential(tunnel i2ptunnel.I2PTunnel, r io.Reader, w io.Writer) error {
	opts := tunnel.Options()
	authType := opts["i2cp.leaseSetAuthType"]
	if authType != "1" && authType != "2" {
		return nil
	}

	typeName := "DH"
	if authType == "2" {
		typeName = "PSK"
	}

	fmt.Fprintf(w, "Tunnel %q requires encrypted LeaseSet authentication (%s).\n", tunnel.Name(), typeName)
	fmt.Fprintf(w, "Enter i2cp.leaseSetPrivKey (Base64): ")

	scanner := bufio.NewScanner(r)
	var line string
	if scanner.Scan() {
		line = strings.TrimSpace(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading LeaseSet key: %w", err)
	}
	if line == "" {
		return fmt.Errorf("i2cp.leaseSetPrivKey is required for %s auth but was not provided", typeName)
	}

	return tunnel.SetOptions(map[string]string{"i2cp.leaseSetPrivKey": line})
}
