package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bridge.conf")
	data := `
# comment
target_host = gost.example.ru
target_port = 443
front_listen = 127.0.0.1:8443
gost_listen = 127.0.0.1:18080
backend = linux-gostengine
`
	if err := os.WriteFile(p, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.TargetHost != "gost.example.ru" {
		t.Errorf("TargetHost = %q", c.TargetHost)
	}
	if c.FrontListen != "127.0.0.1:8443" {
		t.Errorf("FrontListen = %q", c.FrontListen)
	}
	if c.ResolveBackend() != BackendGostEngine {
		t.Errorf("backend = %q", c.ResolveBackend())
	}
	if c.EffectiveSNI() != "gost.example.ru" {
		t.Errorf("EffectiveSNI = %q", c.EffectiveSNI())
	}
}

func TestLoadUnknownKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bridge.conf")
	os.WriteFile(p, []byte("bogus = 1\n"), 0644)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestValidate(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err == nil {
		t.Fatal("expected error when target_host empty")
	}
	c.TargetHost = "x"
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Defaults()
	c.TargetHost = "h.example"
	c.SNI = "sni.example"
	p := filepath.Join(dir, "out.conf")
	if err := os.WriteFile(p, []byte(c.Marshal()), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetHost != "h.example" || got.SNI != "sni.example" {
		t.Errorf("round trip lost fields: %+v", got)
	}
}
