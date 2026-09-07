//go:build windows

package proc

import (
	"os"
	"os/exec"
	"syscall"
)

func detach(cmd *exec.Cmd) {
	// CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS so the child outlives the CLI.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200 | 0x00000008,
	}
}

func isRunning(pid int) bool {
	// os.FindProcess on Windows opens a handle; a failed open means gone.
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal(0) is unsupported on Windows; probe via a harmless wait check.
	// Sending Signal on Windows only supports Kill, so we treat a successful
	// handle open as "running". Release the handle afterwards.
	defer p.Release()
	return true
}

func kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
