// Package bridge renders stunnel configs and orchestrates the two legs.
package bridge

import (
	"fmt"
	"strings"

	"github.com/mitra1n/gost-tls-bridge/internal/config"
)

// FrontConf renders the frontend stunnel config: it terminates the plaintext
// TLS from Burp using our relay cert and forwards cleartext to the GOST leg.
func FrontConf(c *config.Config, logPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "; gost-tls-bridge frontend leg (Burp -> here)\n")
	fmt.Fprintf(&b, "foreground = yes\n")
	fmt.Fprintf(&b, "output = %s\n", logPath)
	fmt.Fprintf(&b, "\n[front]\n")
	fmt.Fprintf(&b, "client = no\n")
	fmt.Fprintf(&b, "accept = %s\n", c.FrontListen)
	fmt.Fprintf(&b, "connect = %s\n", c.GostListen)
	fmt.Fprintf(&b, "cert = %s\n", c.RelayCert)
	return b.String()
}

// GostConf renders the outbound GOST-TLS leg for the resolved backend.
func GostConf(c *config.Config, logPath string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "; gost-tls-bridge GOST leg (here -> %s)\n", c.TargetHost)
	fmt.Fprintf(&b, "foreground = yes\n")
	fmt.Fprintf(&b, "output = %s\n", logPath)

	switch c.ResolveBackend() {
	case config.BackendMSSPI:
		fmt.Fprintf(&b, "\n[gost-out]\n")
		fmt.Fprintf(&b, "client = yes\n")
		fmt.Fprintf(&b, "msspi = yes\n")
		fmt.Fprintf(&b, "accept = %s\n", c.GostListen)
		fmt.Fprintf(&b, "connect = %s:%d\n", c.TargetHost, c.TargetPort)
		fmt.Fprintf(&b, "sni = %s\n", c.EffectiveSNI())
	case config.BackendGostEngine:
		// Requires an stunnel built against OpenSSL with gost-engine loaded.
		fmt.Fprintf(&b, "engine = gost\n")
		fmt.Fprintf(&b, "\n[gost-out]\n")
		fmt.Fprintf(&b, "client = yes\n")
		fmt.Fprintf(&b, "sslVersion = TLSv1.2\n")
		fmt.Fprintf(&b, "ciphers = GOST2012-GOST8912-GOST8912\n")
		fmt.Fprintf(&b, "accept = %s\n", c.GostListen)
		fmt.Fprintf(&b, "connect = %s:%d\n", c.TargetHost, c.TargetPort)
		fmt.Fprintf(&b, "sni = %s\n", c.EffectiveSNI())
	default:
		return "", fmt.Errorf("unsupported backend %q", c.ResolveBackend())
	}
	return b.String(), nil
}
