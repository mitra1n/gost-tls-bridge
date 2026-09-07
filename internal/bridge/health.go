package bridge

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mitra1n/gost-tls-bridge/internal/config"
)

// PortOpen reports whether a TCP listener is accepting on addr.
func PortOpen(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// CurlThroughBridge performs an end-to-end HTTPS GET through the frontend leg,
// spoofing the Host header to the target, mirroring the manual curl check.
// It returns the HTTP status line on success.
func CurlThroughBridge(c *config.Config) (string, error) {
	host, _, err := net.SplitHostPort(c.FrontListen)
	if err != nil {
		return "", fmt.Errorf("front_listen: %w", err)
	}
	url := fmt.Sprintf("https://%s/", c.FrontListen)

	tr := &http.Transport{
		// Front leg presents our self-signed relay cert; skip verification.
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         c.TargetHost,
		},
		DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}
	client := &http.Client{Transport: tr, Timeout: 15 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Host = c.TargetHost
	_ = host

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return strings.TrimSpace(resp.Status), nil
}
