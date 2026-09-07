package webui

import (
	"os/exec"
	"runtime"
)

// OpenBrowser best-effort opens url in the default browser. Errors are ignored
// because the panel is still reachable manually at the printed URL.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
