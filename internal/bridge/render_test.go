package bridge

import (
	"strings"
	"testing"

	"github.com/mitra1n/gost-tls-bridge/internal/config"
)

func TestGostConfMSSPI(t *testing.T) {
	c := config.Defaults()
	c.TargetHost = "gost.example.ru"
	c.Backend = config.BackendMSSPI
	out, err := GostConf(c, "gost.log")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"msspi = yes", "connect = gost.example.ru:443", "sni = gost.example.ru"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGostConfGostEngine(t *testing.T) {
	c := config.Defaults()
	c.TargetHost = "h"
	c.Backend = config.BackendGostEngine
	out, err := GostConf(c, "gost.log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "engine = gost") || !strings.Contains(out, "GOST2012-GOST8912-GOST8912") {
		t.Errorf("gost-engine conf wrong:\n%s", out)
	}
}

func TestFrontConf(t *testing.T) {
	c := config.Defaults()
	c.RelayCert = "/x/relay.pem"
	out := FrontConf(c, "front.log")
	if !strings.Contains(out, "cert = /x/relay.pem") || !strings.Contains(out, "client = no") {
		t.Errorf("front conf wrong:\n%s", out)
	}
}
