//go:build windows

package hdr

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

const videoSettingsKey = `Software\Microsoft\Windows\CurrentVersion\VideoSettings`

// WindowsController uses the same Windows-provided Win+Alt+B mechanism users
// use manually. It never writes the EnableHDR registry value; that value is
// only used as a best-effort post-switch state probe.
type WindowsController struct {
	settleDelay time.Duration
	retries     int
}

func NewWindowsController() *WindowsController {
	return &WindowsController{settleDelay: 500 * time.Millisecond, retries: 5}
}

func (c *WindowsController) State(_ context.Context) (State, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, videoSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		if state, ok := advancedColorState(); ok {
			return state, nil
		}
		return Unknown, fmt.Errorf("打开 HDR 设置: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("EnableHDR")
	if err != nil {
		return Unknown, fmt.Errorf("读取 HDR 状态: %w", err)
	}
	if value == 0 {
		return Off, nil
	}
	if value == 1 {
		return On, nil
	}
	return Unknown, fmt.Errorf("未知 HDR 注册表值: %d", value)
}

func (c *WindowsController) Set(ctx context.Context, target State) (State, error) {
	if target != On && target != Off {
		return Unknown, fmt.Errorf("无效 HDR 目标状态: %s", target)
	}
	current, err := c.State(ctx)
	if err == nil && current == target {
		return current, nil
	}
	// Keep the same state source as the original Python client. Windows' HDR
	// shortcut changes the display, while this per-user value is what the
	// shell and older builds expose for polling.
	if key, keyErr := registry.OpenKey(registry.CURRENT_USER, videoSettingsKey, registry.SET_VALUE); keyErr == nil {
		value := uint32(0)
		if target == On {
			value = 1
		}
		_ = key.SetDWordValue("EnableHDR", value)
		_ = key.Close()
	}
	if err := sendToggleHotkey(); err != nil {
		return Unknown, err
	}
	for attempt := 0; attempt < c.retries; attempt++ {
		select {
		case <-ctx.Done():
			return Unknown, ctx.Err()
		case <-time.After(c.settleDelay):
		}
		state, stateErr := c.State(ctx)
		if stateErr == nil && state == target {
			return state, nil
		}
	}
	state, stateErr := c.State(ctx)
	if stateErr != nil {
		return Unknown, fmt.Errorf("HDR 切换后无法验证: %w", stateErr)
	}
	return state, fmt.Errorf("HDR 切换后状态为 %s，期望 %s", state, target)
}

const (
	vkLWin              = 0x5B
	vkLAlt              = 0xA4
	vkB                 = 0x42
	keyEventKeyUp       = 0x0002
	keyEventExtendedKey = 0x0001
)

var keybdEvent = syscall.NewLazyDLL("user32.dll").NewProc("keybd_event")

func sendKey(vk, flags uintptr) error {
	_, _, callErr := keybdEvent.Call(vk, 0, flags, 0)
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}

func sendToggleHotkey() error {
	if err := sendKey(vkLWin, keyEventExtendedKey); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := sendKey(vkLAlt, keyEventExtendedKey); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := sendKey(vkB, 0); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	if err := sendKey(vkB, keyEventKeyUp); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := sendKey(vkLAlt, keyEventKeyUp|keyEventExtendedKey); err != nil {
		return err
	}
	return sendKey(vkLWin, keyEventKeyUp|keyEventExtendedKey)
}
