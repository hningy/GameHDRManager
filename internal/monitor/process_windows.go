//go:build windows

package monitor

import (
	"os/exec"
	"syscall"
)

func prepareHiddenProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
