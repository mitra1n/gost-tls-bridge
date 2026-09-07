// Command gost-tls-bridge is a cross-platform launcher for a GOST-TLS MITM
// bridge in front of Burp Suite. It orchestrates two stunnel legs:
//
//	Burp --(ordinary TLS, relay cert)--> front_listen --plaintext--> gost_listen --(GOST TLS)--> target
//
// The GOST leg uses CryptoPro CSP via stunnel-msspi on Windows, or OpenSSL
// gost-engine via stunnel on Linux.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mitra1n/gost-tls-bridge/internal/bridge"
	"github.com/mitra1n/gost-tls-bridge/internal/cert"
	"github.com/mitra1n/gost-tls-bridge/internal/config"
	"github.com/mitra1n/gost-tls-bridge/internal/webui"
)

const usage = `gost-tls-bridge — ГОСТ TLS MITM-мост для Burp

Usage:
  gost-tls-bridge <command> [flags]

Commands:
  init       создать workdir, конфиг и relay-серт
  certgen    (пере)сгенерировать relay-серт
  start      отрендерить конфиги и поднять оба плеча
  stop       остановить оба плеча
  restart    stop + start
  status     показать состояние процессов и портов
  check      end-to-end проверка сквозь мост (HTTPS GET)
  burp       напечатать шаги настройки Burp
  gui        локальная веб-панель управления (открывает браузер)
  render     только отрендерить stunnel-конфиги (без запуска)
  version    версия

Global flags:
  -c <path>  путь к конфигу (по умолчанию <workdir>/bridge.conf)
`

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	// The subcommand is the first non-flag argument; everything else is a
	// flag and may appear before or after it.
	cmd, rest := splitCommand(os.Args[1:])
	if cmd == "" {
		fmt.Print(usage)
		os.Exit(2)
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	cfgPath := fs.String("c", "", "path to config file")
	// init-only flags
	target := fs.String("target", "", "target host (init)")
	port := fs.Int("port", 443, "target port (init)")
	sni := fs.String("sni", "", "SNI override (init)")
	addr := fs.String("addr", "127.0.0.1:8765", "listen address (gui)")
	noOpen := fs.Bool("no-open", false, "do not open the browser (gui)")
	_ = fs.Parse(rest)

	resolvedCfgPath := func() string {
		if *cfgPath != "" {
			return *cfgPath
		}
		return filepath.Join(config.Defaults().WorkDir, "bridge.conf")
	}

	switch cmd {
	case "version", "-v", "--version":
		fmt.Println("gost-tls-bridge", version)
	case "init":
		mustOK(cmdInit(resolvedCfgPath(), *target, *port, *sni))
	case "certgen":
		mustOK(withConfig(resolvedCfgPath(), cmdCertgen))
	case "render":
		mustOK(withConfig(resolvedCfgPath(), cmdRender))
	case "start":
		mustOK(withConfig(resolvedCfgPath(), cmdStart))
	case "stop":
		mustOK(withConfig(resolvedCfgPath(), cmdStop))
	case "restart":
		mustOK(withConfig(resolvedCfgPath(), func(c *config.Config) error {
			_ = bridge.Stop(c)
			time.Sleep(300 * time.Millisecond)
			return cmdStart(c)
		}))
	case "status":
		mustOK(withConfig(resolvedCfgPath(), cmdStatus))
	case "check":
		mustOK(withConfig(resolvedCfgPath(), cmdCheck))
	case "burp":
		mustOK(withConfig(resolvedCfgPath(), cmdBurp))
	case "gui":
		mustOK(cmdGUI(resolvedCfgPath(), *addr, *noOpen))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		fmt.Print(usage)
		os.Exit(2)
	}
}

// valueFlags are flags that consume the following argument as their value,
// so that token must not be mistaken for the subcommand.
var valueFlags = map[string]bool{
	"-c": true, "--c": true,
	"-target": true, "--target": true,
	"-port": true, "--port": true,
	"-sni": true, "--sni": true,
}

// splitCommand pulls the first non-flag token out of args as the subcommand,
// returning it plus the remaining args. Flags may appear before or after the
// command; a value-taking flag written as "-c path" carries its value with it.
func splitCommand(args []string) (cmd string, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 0 && a[0] == '-' {
			// Skip the value token of a space-separated value flag.
			if valueFlags[a] && i+1 < len(args) {
				i++
			}
			continue
		}
		rest = append(append([]string{}, args[:i]...), args[i+1:]...)
		return a, rest
	}
	return "", args
}

func withConfig(path string, fn func(*config.Config) error) error {
	c, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("load config %s: %w (run `init` first?)", path, err)
	}
	if err := c.Validate(); err != nil {
		return err
	}
	return fn(c)
}

func cmdInit(cfgPath, target string, port int, sni string) error {
	c := config.Defaults()
	if target != "" {
		c.TargetHost = target
	}
	c.TargetPort = port
	c.SNI = sni

	if err := os.MkdirAll(c.WorkDir, 0755); err != nil {
		return err
	}
	if c.TargetHost == "" {
		fmt.Println("! target_host не задан — впиши его в", cfgPath, "(флаг -target для авто)")
	}
	if err := os.WriteFile(cfgPath, []byte(c.Marshal()), 0644); err != nil {
		return err
	}
	fmt.Println("конфиг:", cfgPath)

	if c.TargetHost != "" {
		if err := cert.GenerateRelayPEM(c.RelayCert, c.TargetHost); err != nil {
			return fmt.Errorf("relay cert: %w", err)
		}
		fmt.Println("relay-серт:", c.RelayCert)
	} else {
		fmt.Println("relay-серт не создан (нет target_host); запусти `certgen` после правки конфига")
	}
	fmt.Println("backend:", c.ResolveBackend())
	return nil
}

func cmdCertgen(c *config.Config) error {
	if err := cert.GenerateRelayPEM(c.RelayCert, c.TargetHost); err != nil {
		return err
	}
	fmt.Println("relay-серт создан:", c.RelayCert, "(SAN:", c.TargetHost+")")
	return nil
}

func cmdRender(c *config.Config) error {
	p, err := bridge.Render(c)
	if err != nil {
		return err
	}
	fmt.Println("front conf:", p.FrontConf)
	fmt.Println("gost  conf:", p.GostConf)
	return nil
}

func cmdStart(c *config.Config) error {
	if _, err := os.Stat(c.RelayCert); err != nil {
		fmt.Println("relay-серт отсутствует — генерирую…")
		if err := cert.GenerateRelayPEM(c.RelayCert, c.TargetHost); err != nil {
			return err
		}
	}
	if _, err := bridge.Start(c); err != nil {
		return err
	}
	fmt.Println("мост запущен")
	time.Sleep(500 * time.Millisecond)
	printStatus(c)
	return nil
}

func cmdStop(c *config.Config) error {
	if err := bridge.Stop(c); err != nil {
		return err
	}
	fmt.Println("мост остановлен")
	return nil
}

func cmdStatus(c *config.Config) error {
	printStatus(c)
	return nil
}

func cmdCheck(c *config.Config) error {
	st := bridge.Probe(c)
	if !st.FrontPortOpen {
		return fmt.Errorf("front-порт %s закрыт — мост не запущен?", c.FrontListen)
	}
	status, err := bridge.CurlThroughBridge(c)
	if err != nil {
		return fmt.Errorf("end-to-end проверка не прошла: %w", err)
	}
	fmt.Println("OK, сервер ответил:", status)
	return nil
}

func cmdBurp(c *config.Config) error {
	fmt.Print(bridge.BurpHints(c))
	return nil
}

func cmdGUI(cfgPath, addr string, noOpen bool) error {
	// The panel loads the config per-request, so it is fine to start even
	// before `init`; it will surface config errors in the UI.
	srv := webui.New(cfgPath)
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	url := "http://" + addr + "/"
	fmt.Println("панель управления:", url, "(config:", cfgPath+")")
	fmt.Println("Ctrl+C для остановки панели (мост продолжит работать).")
	if !noOpen {
		webui.OpenBrowser(url)
	}
	return httpSrv.Serve(ln)
}

func printStatus(c *config.Config) {
	st := bridge.Probe(c)
	yn := func(b bool) string {
		if b {
			return "up"
		}
		return "down"
	}
	fmt.Printf("target:  %s:%d (backend %s)\n", c.TargetHost, c.TargetPort, c.ResolveBackend())
	fmt.Printf("gost  leg: proc %-4s  port %s %s\n", yn(st.GostUp), c.GostListen, yn(st.GostPortOpen))
	fmt.Printf("front leg: proc %-4s  port %s %s\n", yn(st.FrontUp), c.FrontListen, yn(st.FrontPortOpen))
}

func mustOK(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
