package main

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// 模拟编辑器：允许读写但不允许删除，旧的直接覆盖回退会在这里截断仍然有效的配置。
func holdConfigForReading(t *testing.T, path string) syscall.Handle {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

func TestBusyConfigNeverTruncatesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.jsonc")
	config := defaultConfig()
	if err := writeConfigFile(path, config); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	handle := holdConfigForReading(t, path)
	config.TrayIcon = "toggle"
	err := writeConfigFile(path, config)
	syscall.CloseHandle(handle)
	if err == nil {
		t.Fatal("文件被占用时应保留原文件并报告保存失败")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("保存失败损坏了原配置")
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".config.jsonc-*.tmp"))
	if len(leftovers) != 0 {
		t.Fatal("失败后遗留了临时配置")
	}
	if err := writeConfigFile(path, config); err != nil {
		t.Fatal("解除占用后应能重试：", err)
	}
}

func TestTransientConfigLockIsRetried(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.jsonc")
	config := defaultConfig()
	if err := writeConfigFile(path, config); err != nil {
		t.Fatal(err)
	}
	handle := holdConfigForReading(t, path)
	done := make(chan struct{})
	go func() { time.Sleep(20 * time.Millisecond); syscall.CloseHandle(handle); close(done) }()
	config.TrayIcon = "toggle"
	err := writeConfigFile(path, config)
	<-done
	if err != nil {
		t.Fatal("短暂占用应自动重试成功：", err)
	}
	data, _ := os.ReadFile(path)
	loaded, err := parseConfig(string(data))
	if err != nil || loaded.TrayIcon != "toggle" {
		t.Fatal("替换后没有得到完整的新配置")
	}
}
