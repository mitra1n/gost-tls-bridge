package bridge

import (
	"fmt"
	"net"
	"strings"

	"github.com/mitra1n/gost-tls-bridge/internal/config"
)

// BurpHints returns step-by-step Burp configuration for the current config,
// derived from the hard-won settings needed to make GOST MITM work.
func BurpHints(c *config.Config) string {
	frontHost, frontPort, _ := net.SplitHostPort(c.FrontListen)
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }

	w("Burp настройка для %s (ГОСТ TLS через мост):", c.TargetHost)
	w("")
	w("1) Network -> DNS: добавить override")
	w("     %s  ->  %s", c.TargetHost, frontHost)
	w("   (Enabled). В Burp 2026 hostname resolution живёт именно здесь,")
	w("   а не в Network -> Connections.")
	w("")
	w("2) Settings -> Network -> TLS:")
	w("   отключить проверку upstream-сертификата, иначе Burp рвёт TLS")
	w("   к нашему self-signed relay (в front.log: SSL_accept unexpected eof).")
	w("")
	w("3) Proxy -> TLS pass-through:")
	w("   убедиться, что %s НЕ в списке pass-through,", c.TargetHost)
	w("   иначе Burp не расшифровывает трафик.")
	w("")
	w("4) Куда шлём трафик:")
	w("   мост слушает %s:%s и презентует relay-серт с SAN=%s.", frontHost, frontPort, c.TargetHost)
	w("   DNS-override выше заворачивает запросы к %s на этот адрес.", c.TargetHost)
	w("")
	w("Проверка без Burp:")
	w("   curl -k https://%s/ -H \"Host: %s\"", c.FrontListen, c.TargetHost)
	return b.String()
}
