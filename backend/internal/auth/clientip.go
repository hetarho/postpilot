package auth

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP resolves the limiter key from the direct peer unless a trusted ingress
// header is explicitly enabled. The rightmost forwarded entry is the one the edge
// appended; entries to its left may have been supplied by the client.
func ClientIP(header http.Header, peerAddr, trustedHeader string) string {
	if trustedHeader != "" {
		values := header.Values(trustedHeader)
		for valueIndex := len(values) - 1; valueIndex >= 0; valueIndex-- {
			parts := strings.Split(values[valueIndex], ",")
			for partIndex := len(parts) - 1; partIndex >= 0; partIndex-- {
				if candidate := strings.TrimSpace(parts[partIndex]); candidate != "" {
					return candidate
				}
			}
		}
	}

	peerAddr = strings.TrimSpace(peerAddr)
	host, _, err := net.SplitHostPort(peerAddr)
	if err == nil {
		return host
	}
	return strings.Trim(peerAddr, "[]")
}
