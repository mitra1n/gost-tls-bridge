// Package config loads and renders the bridge configuration.
//
// The config file is a tiny, dependency-free "key = value" format with
// '#' comments, so the resulting binary stays a single static executable.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Backend selects the outbound GOST-TLS implementation.
type Backend string

const (
	BackendAuto       Backend = "auto"             // pick per-OS
	BackendMSSPI      Backend = "windows-msspi"    // CryptoPro CSP via stunnel-msspi
	BackendGostEngine Backend = "linux-gostengine" // OpenSSL gost-engine via stunnel
)

// Config is the fully-resolved bridge configuration.
type Config struct {
	// Target GOST-TLS server.
	TargetHost string
	TargetPort int
	SNI        string // defaults to TargetHost

	// Listeners.
	FrontListen string // where Burp connects (plaintext TLS with our relay cert)
	GostListen  string // internal plaintext hop between front and gost legs

	// Relay certificate presented to Burp (self-signed).
	RelayCert string // combined cert+key PEM

	// Backend selection and stunnel binaries.
	Backend      Backend
	StunnelBin   string // ordinary stunnel+OpenSSL (frontend leg)
	StunnelMSSPI string // stunnel-msspi (windows gost leg)

	// Working directory holding rendered confs, logs and pidfiles.
	WorkDir string
}

// Defaults returns a Config populated with sensible per-OS defaults.
func Defaults() *Config {
	wd := defaultWorkDir()
	c := &Config{
		TargetPort:  443,
		FrontListen: "127.0.0.1:443",
		GostListen:  "127.0.0.1:18080",
		RelayCert:   filepath.Join(wd, "relay.pem"),
		Backend:     BackendAuto,
		WorkDir:     wd,
	}
	if runtime.GOOS == "windows" {
		c.StunnelBin = `C:\gost\ossl_all\bin\stunnel.exe`
		c.StunnelMSSPI = `C:\gost\stunnel-msspi.exe`
	} else {
		c.StunnelBin = "stunnel"
		c.StunnelMSSPI = "stunnel" // gost-engine build
	}
	return c
}

func defaultWorkDir() string {
	if runtime.GOOS == "windows" {
		return `C:\gost`
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".gost-tls-bridge")
	}
	return ".gost-tls-bridge"
}

// ResolveBackend turns BackendAuto into a concrete backend for this OS.
func (c *Config) ResolveBackend() Backend {
	if c.Backend != BackendAuto {
		return c.Backend
	}
	if runtime.GOOS == "windows" {
		return BackendMSSPI
	}
	return BackendGostEngine
}

// EffectiveSNI returns the SNI to send (defaults to the target host).
func (c *Config) EffectiveSNI() string {
	if c.SNI != "" {
		return c.SNI
	}
	return c.TargetHost
}

// Validate checks the config is usable before starting the bridge.
func (c *Config) Validate() error {
	if c.TargetHost == "" {
		return fmt.Errorf("target_host is required (run `init` or edit the config)")
	}
	if c.TargetPort <= 0 || c.TargetPort > 65535 {
		return fmt.Errorf("target_port %d is out of range", c.TargetPort)
	}
	if c.FrontListen == "" || c.GostListen == "" {
		return fmt.Errorf("front_listen and gost_listen must be set")
	}
	return nil
}

// Load reads a config file, layering it over Defaults().
func Load(path string) (*Config, error) {
	c := Defaults()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key = value", path, line)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if err := c.set(key, val); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) set(key, val string) error {
	switch key {
	case "target_host":
		c.TargetHost = val
	case "target_port":
		p, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("target_port: %w", err)
		}
		c.TargetPort = p
	case "sni":
		c.SNI = val
	case "front_listen":
		c.FrontListen = val
	case "gost_listen":
		c.GostListen = val
	case "relay_cert":
		c.RelayCert = val
	case "backend":
		c.Backend = Backend(val)
	case "stunnel_bin":
		c.StunnelBin = val
	case "stunnel_msspi_bin":
		c.StunnelMSSPI = val
	case "workdir":
		c.WorkDir = val
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

// Marshal renders the config back to the on-disk key = value format.
func (c *Config) Marshal() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# gost-tls-bridge configuration\n")
	fmt.Fprintf(&b, "# Target GOST-TLS server to reach through the bridge.\n")
	fmt.Fprintf(&b, "target_host = %s\n", c.TargetHost)
	fmt.Fprintf(&b, "target_port = %d\n", c.TargetPort)
	fmt.Fprintf(&b, "# SNI to send; leave blank to use target_host.\n")
	fmt.Fprintf(&b, "sni = %s\n\n", c.SNI)
	fmt.Fprintf(&b, "# Where Burp points its upstream proxy (plaintext TLS, our relay cert).\n")
	fmt.Fprintf(&b, "front_listen = %s\n", c.FrontListen)
	fmt.Fprintf(&b, "# Internal plaintext hop between the front and the GOST leg.\n")
	fmt.Fprintf(&b, "gost_listen = %s\n\n", c.GostListen)
	fmt.Fprintf(&b, "relay_cert = %s\n\n", c.RelayCert)
	fmt.Fprintf(&b, "# backend: auto | windows-msspi | linux-gostengine\n")
	fmt.Fprintf(&b, "backend = %s\n", c.Backend)
	fmt.Fprintf(&b, "stunnel_bin = %s\n", c.StunnelBin)
	fmt.Fprintf(&b, "stunnel_msspi_bin = %s\n\n", c.StunnelMSSPI)
	fmt.Fprintf(&b, "workdir = %s\n", c.WorkDir)
	return b.String()
}

// StunnelMSSPIOrBin returns the binary to run the GOST (outbound) leg for the
// resolved backend: stunnel-msspi on Windows, the gost-engine stunnel on Linux.
func (c *Config) StunnelMSSPIOrBin() string {
	if c.ResolveBackend() == BackendMSSPI {
		return c.StunnelMSSPI
	}
	return c.StunnelBin
}
