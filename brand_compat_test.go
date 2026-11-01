package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProxiBrandUpgradeOnlyForStartup(t *testing.T) {
	for _, command := range []string{"status", "help", "off", "on", "use", "--core-host"} {
		if preparesBrandUpgrade([]string{command}) {
			t.Errorf("%s 不应中断正在运行的旧版", command)
		}
	}
	for _, command := range []string{"settings", "float", "--autostart", "--updated-from=123", "proxi://settings"} {
		if !preparesBrandUpgrade([]string{command}) {
			t.Errorf("%s 应进入更名升级", command)
		}
	}
	if !preparesBrandUpgrade(nil) {
		t.Error("正常启动应迁移数据")
	}
}

func TestProxiLegacyDataMigration(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, legacyAppName), filepath.Join(root, appName)
	if err := os.MkdirAll(filepath.Join(source, "core", "subscriptions"), 0700); err != nil {
		t.Fatal(err)
	}
	config := []byte("\xef\xbb\xbf// ProxySwitch 配置\n{\"profiles\":[{\"name\":\"ProxySwitch 用户自己起的名字\",\"server\":\"127.0.0.1:7890\"}],\"tray_click\":\"settings\"}")
	for name, data := range map[string][]byte{configFileName: config, stateFileName: []byte(`{"enabled":true,"profile_id":"p1"}`), legacyLogFileName: []byte("旧日志"), "core/subscriptions/p1.yaml": []byte("保留节点缓存")} {
		if err := os.WriteFile(filepath.Join(source, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	changed, err := migrateLegacyData(source, destination)
	if err != nil || !changed {
		t.Fatalf("应完成迁移：%v %v", changed, err)
	}
	newConfig, _ := os.ReadFile(filepath.Join(destination, configFileName))
	before, err := parseConfig(string(config))
	if err != nil {
		t.Fatal(err)
	}
	after, err := parseConfig(string(newConfig))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("更名不应改变用户的配置、名字或点击偏好")
	}
	if !bytes.Contains(newConfig, []byte("// Proxi 配置")) {
		t.Error("程序生成的注释应显示新名称")
	}
	original, _ := os.ReadFile(filepath.Join(source, configFileName))
	if !bytes.Equal(original, config) {
		t.Error("旧数据必须完整保留")
	}
	for _, name := range []string{stateFileName, logFileName, "core/subscriptions/p1.yaml"} {
		if !fileExists(filepath.Join(destination, name)) {
			t.Errorf("迁移遗漏 %s", name)
		}
	}
	if changed, err := migrateLegacyData(source, destination); err != nil || changed {
		t.Error("重复启动不应覆盖已经迁移好的数据")
	}
}

func TestProxiMigrationNeverOverwritesOtherData(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, legacyAppName), filepath.Join(root, appName)
	_ = os.MkdirAll(source, 0700)
	_ = os.WriteFile(filepath.Join(source, configFileName), []byte(`{}`), 0600)
	_ = os.MkdirAll(destination, 0700)
	_ = os.WriteFile(filepath.Join(destination, "existing.txt"), []byte("用户的文件"), 0600)
	if changed, err := migrateLegacyData(source, destination); err == nil || changed {
		t.Error("遇到已有内容应停止，不能覆盖")
	}
	data, _ := os.ReadFile(filepath.Join(destination, "existing.txt"))
	if string(data) != "用户的文件" {
		t.Error("已有文件改变了")
	}
	if fileExists(filepath.Join(destination, configFileName)) {
		t.Error("失败时不应留下半份配置")
	}
	left, _ := filepath.Glob(filepath.Join(root, ".Proxi-migration-*"))
	if len(left) != 0 {
		t.Error("临时目录没有清理")
	}
}

func TestProxiLegacyLinksAndSyncFolder(t *testing.T) {
	for _, scheme := range []string{linkScheme, legacyLinkScheme} {
		link := scheme + "://use?name=公司代理"
		request, err := parseLink(link)
		if !isLink(link) || err != nil || request.Command != "use" || request.Argument != "公司代理" {
			t.Errorf("旧链接应继续可用：%s %v", link, err)
		}
	}
	root := t.TempDir()
	for _, name := range []string{syncFolderVariable, "OneDriveConsumer", "OneDriveCommercial"} {
		t.Setenv(name, "")
	}
	t.Setenv("OneDrive", root)
	legacy := filepath.Join(root, legacyAppName)
	_ = os.MkdirAll(legacy, 0700)
	_ = os.WriteFile(filepath.Join(legacy, syncFileName), []byte(`{}`), 0600)
	if defaultSyncFolder() != legacy {
		t.Error("不能把已有多机同步分成两份")
	}
	current := filepath.Join(root, appName)
	_ = os.MkdirAll(current, 0700)
	_ = os.WriteFile(filepath.Join(current, syncFileName), []byte(`{}`), 0600)
	if defaultSyncFolder() != current {
		t.Error("已有新目录时应使用新目录")
	}
}
