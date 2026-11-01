//go:build windows

package main

import (
	"errors"
	"os"
	"strings"
)

// 开机自启：写 HKCU\...\Run，不需要管理员权限。

const (
	runKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "Proxi"
)

func autostartCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return `"` + executable + `" --autostart`, nil
}

func isAutostartEnabled() bool {
	return strings.TrimSpace(readRegistryString(hkeyCurrentUser, runKey, runValueName)) != "" || legacyOwnedCommand(readRegistryString(hkeyCurrentUser, runKey, legacyAppName))
}

// refreshAutostartPath 在已开启自启、但记录的不是当前 exe 路径时（exe 被挪动或更新到别处）改成当前路径。
func refreshAutostartPath() {
	current := strings.TrimSpace(readRegistryString(hkeyCurrentUser, runKey, runValueName))
	legacy := strings.TrimSpace(readRegistryString(hkeyCurrentUser, runKey, legacyAppName))
	command, err := autostartCommand()
	if err != nil || current == "" && !legacyOwnedCommand(legacy) || strings.EqualFold(current, command) && legacy == "" {
		return
	}
	_ = setAutostart(true)
}

func setAutostart(enabled bool) error {
	key, err := createRegistryKey(hkeyCurrentUser, runKey)
	if err != nil {
		return err
	}
	defer key.Close()
	legacy, _ := key.String(legacyAppName)
	removeLegacy := func() error {
		if legacyOwnedCommand(legacy) {
			return key.DeleteValue(legacyAppName)
		}
		return nil
	}
	if !enabled {
		return errors.Join(key.DeleteValue(runValueName), removeLegacy())
	}
	command, err := autostartCommand()
	if err != nil {
		return err
	}
	if err := key.SetString(runValueName, command); err != nil {
		return err
	}
	return removeLegacy()
}
