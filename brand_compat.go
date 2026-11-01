package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// 旧名称只用于识别已有数据和系统登记，不作为当前产品名称显示。
const (
	legacyAppName           = "ProxySwitch"
	legacyLinkScheme        = "proxyswitch"
	legacyEnvironmentPrefix = "PROXYSWITCH_"
	legacyLogFileName       = "proxyswitch.log"
)

func preparesBrandUpgrade(arguments []string) bool {
	if len(arguments) == 0 {
		return true
	}
	argument := strings.ToLower(arguments[0])
	if strings.HasPrefix(argument, "--updated-from=") {
		return true
	}
	if isLink(argument) {
		request, err := parseLink(arguments[0])
		if err != nil {
			return false
		}
		argument = request.Command
	}
	switch argument {
	case "--autostart", "settings", "update", "diagnose", "float", "import":
		return true
	default:
		return false
	}
}

func init() {
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if suffix, ok := strings.CutPrefix(key, legacyEnvironmentPrefix); ok {
			if _, exists := os.LookupEnv("PROXI_" + suffix); !exists {
				_ = os.Setenv("PROXI_"+suffix, value)
			}
		}
	}
}

func configBaseDir() string {
	if base := os.Getenv("APPDATA"); base != "" {
		return base
	}
	if base, err := os.UserConfigDir(); err == nil {
		return base
	}
	return "."
}

// migrateLegacyData 先完整复制到同级临时目录，再一次性发布。原目录始终保留；已有 Proxi 配置优先。
func migrateLegacyData(source, destination string) (bool, error) {
	if fileExists(filepath.Join(destination, configFileName)) || !fileExists(filepath.Join(source, configFileName)) {
		return false, nil
	}
	parent, err := filepath.Abs(filepath.Dir(destination))
	if err != nil {
		return false, err
	}
	if err = os.MkdirAll(parent, 0700); err != nil {
		return false, err
	}
	stage, err := os.MkdirTemp(parent, ".Proxi-migration-")
	if err != nil {
		return false, err
	}
	defer func() {
		// 只清理本次创建且位于目标父目录下的临时目录。
		if relative, err := filepath.Rel(parent, stage); err == nil && filepath.Dir(relative) == "." && strings.HasPrefix(relative, ".Proxi-migration-") {
			_ = os.RemoveAll(stage)
		}
	}()
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == legacyLogFileName && !fileExists(filepath.Join(source, logFileName)) {
			relative = logFileName
		}
		target := filepath.Join(stage, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("旧数据包含链接，请先检查：%s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持迁移的文件类型：%s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if relative == configFileName {
			data, readErr := io.ReadAll(input)
			if readErr == nil {
				_, readErr = output.Write(proxiConfigComments(data))
			}
			err = readErr
		} else {
			_, err = io.Copy(output, input)
		}
		return errors.Join(err, output.Close())
	})
	if err != nil {
		return false, err
	}
	if entries, err := os.ReadDir(destination); err == nil {
		if len(entries) != 0 {
			return false, errors.New("Proxi 数据目录已有其他内容，未覆盖任何文件")
		}
		if err := os.Remove(destination); err != nil {
			return false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.Rename(stage, destination); err != nil {
		if fileExists(filepath.Join(destination, configFileName)) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func proxiConfigComments(data []byte) []byte {
	bom := bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf})
	lines := strings.Split(string(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})), "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			lines[index] = strings.ReplaceAll(strings.ReplaceAll(line, legacyAppName, appName), legacyLinkScheme+"://", linkScheme+"://")
		}
	}
	result := []byte(strings.Join(lines, "\n"))
	if bom {
		result = append([]byte{0xef, 0xbb, 0xbf}, result...)
	}
	return result
}
