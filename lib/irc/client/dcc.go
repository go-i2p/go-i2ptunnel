package ircclient

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DCC CTCP message patterns. The body is the text that follows the "\x01"
// prefix and precedes the optional trailing "\x01".
var (
	reDCCSend = regexp.MustCompile(`(?i)^DCC\s+SEND\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s*$`)
	reDCCChat = regexp.MustCompile(`(?i)^DCC\s+CHAT\s+chat\s+(\d+)\s+(\d+)\s*$`)
)

// isPrivateIP reports whether ipInt — a 32-bit big-endian integer as used in
// the DCC protocol — falls in RFC-1918 or loopback address space.
func isPrivateIP(ipInt uint32) bool {
	a := byte(ipInt >> 24)
	b := byte(ipInt >> 16)
	return a == 127 || // 127.0.0.0/8  loopback
		a == 10 || // 10.0.0.0/8   RFC-1918
		(a == 172 && b >= 16 && b <= 31) || // 172.16.0.0/12 RFC-1918
		(a == 192 && b == 168) // 192.168.0.0/16 RFC-1918
}

// validateDCCPort returns an error when port falls outside the permitted range
// of 1024–65535.
func validateDCCPort(port int) error {
	if port < 1024 || port > 65535 {
		return fmt.Errorf("invalid DCC port %d: must be between 1024 and 65535", port)
	}
	return nil
}

// filterDCCRequest validates the parameters in a DCC CTCP message body.
func filterDCCRequest(body string) error {
	body = strings.TrimSpace(body)
	if end := strings.IndexByte(body, '\x01'); end >= 0 {
		body = body[:end]
	}

	if m := reDCCSend.FindStringSubmatch(body); m != nil {
		return validateDCCIPPort("DCC SEND", m[2], m[3])
	}
	if m := reDCCChat.FindStringSubmatch(body); m != nil {
		return validateDCCIPPort("DCC CHAT", m[1], m[2])
	}
	return nil
}

// validateDCCIPPort checks that the ip integer and port string from a DCC message are safe.
func validateDCCIPPort(kind, ipStr, portStr string) error {
	ipInt, err := strconv.ParseUint(ipStr, 10, 32)
	if err != nil {
		return fmt.Errorf("%s: cannot parse IP %q", kind, ipStr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("%s: cannot parse port %q", kind, portStr)
	}
	if isPrivateIP(uint32(ipInt)) {
		return fmt.Errorf("%s blocked: private IP address", kind)
	}
	return validateDCCPort(port)
}
