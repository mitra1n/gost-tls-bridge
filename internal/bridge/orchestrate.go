package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mitra1n/gost-tls-bridge/internal/config"
	"github.com/mitra1n/gost-tls-bridge/internal/proc"
)

const (
	nameFront = "front"
	nameGost  = "gost"
)

// Paths bundles the on-disk artefacts for a running bridge.
type Paths struct {
	FrontConf string
	GostConf  string
	FrontLog  string
	GostLog   string
	FrontPid  string
	GostPid   string
}

func paths(c *config.Config) Paths {
	j := func(n string) string { return filepath.Join(c.WorkDir, n) }
	return Paths{
		FrontConf: j("front.conf"),
		GostConf:  j("gost-out.conf"),
		FrontLog:  j("front.log"),
		GostLog:   j("gost.log"),
		FrontPid:  proc.PidFile(c.WorkDir, nameFront),
		GostPid:   proc.PidFile(c.WorkDir, nameGost),
	}
}

// Render writes both stunnel config files from the current config.
func Render(c *config.Config) (Paths, error) {
	p := paths(c)
	if err := os.MkdirAll(c.WorkDir, 0755); err != nil {
		return p, err
	}
	if err := os.WriteFile(p.FrontConf, []byte(FrontConf(c, p.FrontLog)), 0644); err != nil {
		return p, err
	}
	gostConf, err := GostConf(c, p.GostLog)
	if err != nil {
		return p, err
	}
	if err := os.WriteFile(p.GostConf, []byte(gostConf), 0644); err != nil {
		return p, err
	}
	return p, nil
}

// Start renders configs and launches both stunnel legs.
func Start(c *config.Config) (Paths, error) {
	p, err := Render(c)
	if err != nil {
		return p, err
	}

	// GOST leg first so the frontend has an upstream to connect to.
	if _, err := proc.Start(c.StunnelMSSPIOrBin(), []string{p.GostConf}, p.GostLog, p.GostPid); err != nil {
		return p, fmt.Errorf("start gost leg: %w", err)
	}
	// Give the listener a moment to bind before health-related callers probe.
	time.Sleep(300 * time.Millisecond)
	if _, err := proc.Start(c.StunnelBin, []string{p.FrontConf}, p.FrontLog, p.FrontPid); err != nil {
		return p, fmt.Errorf("start front leg: %w", err)
	}
	return p, nil
}

// Stop terminates both legs.
func Stop(c *config.Config) error {
	p := paths(c)
	var firstErr error
	for _, pid := range []string{p.FrontPid, p.GostPid} {
		if err := proc.Stop(pid); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Status describes the live state of both legs.
type Status struct {
	FrontPid, GostPid int
	FrontUp, GostUp   bool
	FrontPortOpen     bool
	GostPortOpen      bool
}

// Probe collects process and port state.
func Probe(c *config.Config) Status {
	p := paths(c)
	fp := proc.ReadPid(p.FrontPid)
	gp := proc.ReadPid(p.GostPid)
	return Status{
		FrontPid:      fp,
		GostPid:       gp,
		FrontUp:       proc.Running(fp),
		GostUp:        proc.Running(gp),
		FrontPortOpen: PortOpen(c.FrontListen, 2*time.Second),
		GostPortOpen:  PortOpen(c.GostListen, 2*time.Second),
	}
}
