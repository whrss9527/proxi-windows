//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

// 隐藏的实例标识和通知身份保持稳定，让旧版命令、卸载程序和通知设置继续有效。
const (
	legacyTrayWindowClass = "ProxySwitchTrayWindow"
	legacyInstanceMutex   = `Local\ProxySwitch-3f1c2a9e`
	legacyToastAppID      = "whrss9527.ProxySwitch"
	legacyLinkBackupValue = "ProxySwitchBackup"
)

func prepareBrandUpgrade() error {
	if window := findRunningInstance(); window != 0 {
		buffer := make([]uint16, 128)
		procGetWindowTextW.Call(window, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		if utf16PointerToString(&buffer[0]) == legacyAppName {
			var pid uint32
			user32.NewProc("GetWindowThreadProcessId").Call(window, uintptr(unsafe.Pointer(&pid)))
			handle, _, err := procOpenProcess.Call(synchronize, 0, uintptr(pid))
			if handle == 0 {
				return fmt.Errorf("等待旧版程序退出：%v", err)
			}
			defer procCloseHandle.Call(handle)
			if sent, _, err := procPostMessageW.Call(window, wmClose, 0, 0); sent == 0 {
				return fmt.Errorf("退出旧版程序：%v", err)
			}
			if result, _, _ := procWaitForSingleObject.Call(handle, uintptr((20 * time.Second).Milliseconds())); result != 0 {
				return fmt.Errorf("旧版程序未能在规定时间内退出")
			}
		}
	}
	if executable, err := os.Executable(); err == nil && fileExists(filepath.Join(filepath.Dir(executable), configFileName)) {
		return nil
	}
	base := configBaseDir()
	_, err := migrateLegacyData(filepath.Join(base, legacyAppName), filepath.Join(base, appName))
	if err != nil {
		return fmt.Errorf("旧版数据迁移没有完成，原文件已保留：%w", err)
	}
	return nil
}

func legacyOwnedCommand(command string) bool {
	base := filepath.Base(commandProgram(command))
	return strings.EqualFold(base, legacyAppName+".exe") || strings.EqualFold(base, legacyAppName+"-arm64.exe")
}

// 原地更新的旧安装版保留位置，更新系统显示名称和快捷方式；新安装包使用 Proxi 目录。
func migrateLegacyInstallInfo() {
	oldKey := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + legacyAppName
	executable, err := os.Executable()
	if err != nil {
		return
	}
	location := readRegistryString(hkeyCurrentUser, oldKey, "InstallLocation")
	if location == "" || !strings.EqualFold(filepath.Clean(location), filepath.Dir(executable)) {
		return
	}
	key, err := createRegistryKey(hkeyCurrentUser, uninstallKey)
	if err != nil {
		return
	}
	defer key.Close()
	shortcut := readRegistryString(hkeyCurrentUser, oldKey, uninstallShortcutValue)
	if strings.EqualFold(filepath.Base(shortcut), legacyAppName+".lnk") {
		newShortcut := filepath.Join(filepath.Dir(shortcut), appName+".lnk")
		if _, err := os.Stat(newShortcut); errors.Is(err, os.ErrNotExist) {
			if os.Rename(shortcut, newShortcut) == nil {
				shortcut = newShortcut
			}
		}
	}
	for name, value := range map[string]string{"DisplayName": appName, "DisplayVersion": appVersion, "InstallLocation": location, "DisplayIcon": executable + ",0", "Publisher": "whrss9527", "UninstallString": `"` + executable + `" --uninstall`, "QuietUninstallString": `"` + executable + `" --uninstall --quiet`, uninstallShortcutValue: shortcut, "URLInfoAbout": repositoryUrl, "HelpLink": repositoryUrl + "/issues"} {
		if key.SetString(name, value) != nil {
			return
		}
	}
	_ = key.SetDword("NoModify", 1)
	_ = key.SetDword("NoRepair", 1)
	_ = deleteRegistryTree(hkeyCurrentUser, oldKey)
}
