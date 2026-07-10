//go:build !windows

package monitor

import "os/exec"

func prepareHiddenProcess(_ *exec.Cmd) {}
