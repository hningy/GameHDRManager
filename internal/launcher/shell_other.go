//go:build !windows

package launcher

import "fmt"

func openWithShell(value string) error {
	return fmt.Errorf("当前平台不支持 ShellExecute: %s", value)
}
