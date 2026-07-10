//go:build windows

package launcher

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func openWithShell(value string) error {
	file, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("open")
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("打开启动地址 %q: %w", value, err)
	}
	return nil
}
