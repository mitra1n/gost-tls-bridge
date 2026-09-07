// Package proc manages long-running child processes (the two stunnel legs)
// with pidfile-based tracking so start/stop/status survive across CLI runs.
package proc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// PidFile returns the pidfile path for a named process inside workdir.
func PidFile(workdir, name string) string {
	return filepath.Join(workdir, name+".pid")
}

// Start launches bin with args, redirecting output to logPath, detaches it,
// and records its PID in pidPath. It does not wait for the process.
func Start(bin string, args []string, logPath, pidPath string) (int, error) {
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("open log: %w", err)
	}
	defer logf.Close()

	cmd := exec.Command(bin, args...)
	cmd.Stdout = logf
	cmd.Stderr = logf
	detach(cmd)

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Release the process so it keeps running after we exit.
	_ = cmd.Process.Release()

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return pid, fmt.Errorf("write pidfile: %w", err)
	}
	return pid, nil
}

// ReadPid reads a PID from a pidfile; returns 0 if absent.
func ReadPid(pidPath string) int {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}

// Running reports whether the process with pid is alive.
func Running(pid int) bool {
	if pid <= 0 {
		return false
	}
	return isRunning(pid)
}

// Stop terminates the process recorded in pidPath and removes the pidfile.
func Stop(pidPath string) error {
	pid := ReadPid(pidPath)
	if pid == 0 {
		return nil
	}
	err := kill(pid)
	_ = os.Remove(pidPath)
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
