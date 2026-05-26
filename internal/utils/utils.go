package utils

import (
	"errors"
	"fmt"
	"net"
	"unicode/utf8"
)

var ErrPrivateIP = errors.New("url must not point to a private or reserved IP address")

func IsPrivateIP(hostname string) error {
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return ErrPrivateIP
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return ErrPrivateIP
		}
		if ip4 := ip.To4(); ip4 != nil && ip4[0] == 0 {
			return ErrPrivateIP
		}
	}
	return nil
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	for maxLen > 0 && !utf8.RuneStart(s[maxLen]) {
		maxLen--
	}
	return s[:maxLen]
}
