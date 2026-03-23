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

// filterDCCRequest validates the parameters in a DCC CTCP message body. The
// body is the text following the "\x01DCC " prefix (and before any trailing
// "\x01"). It returns nil when the parameters are technically valid (public IP,
// safe port) and a descriptive error when they violate the private-IP or
// port-range rules. A nil return does NOT mean the message should be forwarded;
// the caller is responsible for deciding whether DCC is permitted at all.
func filterDCCRequest(body string) error {
	body = strings.TrimSpace(body)
	if end := strings.IndexByte(body, '\x01'); end >= 0 {
		body = body[:end]
	}

	if m := reDCCSend.FindStringSubmatch(body); m != nil {
		ipInt, err := strconv.ParseUint(m[2], 10, 32)
		if err != nil {
			return fmt.Errorf("DCC SEND: cannot parse IP %q", m[2])
		}
		port, err := strconv.Atoi(m[3])
		if err != nil {
			return fmt.Errorf("DCC SEND: cannot parse port %q", m[3])
		}
		if isPrivateIP(uint32(ipInt)) {
			return fmt.Errorf("DCC SEND blocked: private IP address")
		}
		return validateDCCPort(port)
	}

	if m := reDCCChat.FindStringSubmatch(body); m != nil {
		ipInt, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			return fmt.Errorf("DCC CHAT: cannot parse IP %q", m[1])
		}
		port, err := strconv.Atoi(m[2])
		if err != nil {
			return fmt.Errorf("DCC CHAT: cannot parse port %q", m[2])
		}
		if isPrivateIP(uint32(ipInt)) {
			return fmt.Errorf("DCC CHAT blocked: private IP address")
		}
		return validateDCCPort(port)
	}

	return nil
}
