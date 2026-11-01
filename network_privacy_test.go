package main

import "testing"

func TestWifiNameReadPolicy(t *testing.T) {
	cases := []struct {
		name   string
		config *Config
		want   bool
	}{
		{"没有配置", nil, false},
		{"默认配置", defaultConfig(), false},
		{"关闭自动切换", &Config{AutoSwitch: AutoSwitch{Rules: []NetRule{{Match: "ssid", Value: "Home"}}}}, false},
		{"没有条件", &Config{AutoSwitch: AutoSwitch{Enabled: true}}, false},
		{"只用网关", &Config{AutoSwitch: AutoSwitch{Enabled: true, Rules: []NetRule{{Match: "gateway", Value: "192.168.1.1"}}}}, false},
		{"只用网卡", &Config{AutoSwitch: AutoSwitch{Enabled: true, Rules: []NetRule{{Match: "adapter", Value: "VPN"}}}}, false},
		{"只用后缀", &Config{AutoSwitch: AutoSwitch{Enabled: true, Rules: []NetRule{{Match: "dns_suffix", Value: "corp.example"}}}}, false},
		{"空名称", &Config{AutoSwitch: AutoSwitch{Enabled: true, Rules: []NetRule{{Match: "ssid", Value: " "}}}}, false},
		{"使用名称条件", &Config{AutoSwitch: AutoSwitch{Enabled: true, Rules: []NetRule{{Match: "ssid", Value: "Home"}}}}, true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := shouldReadWifiName(item.config); got != item.want {
				t.Fatalf("读取策略 = %v，期望 %v", got, item.want)
			}
		})
	}
}

func TestNetworkCollectionSkipsWifi(t *testing.T) {
	wifiCalls := 0
	wifi := func() ([]string, string) { wifiCalls++; return []string{"Home"}, "" }
	adapters := func() ([]NetworkAdapter, []NetworkInterface) {
		return []NetworkAdapter{{Name: "WLAN", Gateway: "192.168.1.1", Wireless: true}}, []NetworkInterface{{Name: "WLAN"}}
	}
	for i := 0; i < 10; i++ {
		info := collectNetworkInfo(false, wifi, adapters)
		if wifiCalls != 0 || info.WifiNameRead || len(info.Ssids) != 0 || !info.Online() || len(info.Interfaces) != 1 {
			t.Fatal("关闭名称读取后仍调用了 Wi-Fi，或丢失了不需要位置权限的网络信息")
		}
	}
	info := collectNetworkInfo(true, wifi, adapters)
	if wifiCalls != 1 || !info.WifiNameRead || len(info.Ssids) != 1 {
		t.Fatal("需要名称时没有且仅查询一次")
	}
	collectNetworkInfo(false, wifi, adapters)
	if wifiCalls != 1 {
		t.Fatal("关闭后继续查询了 Wi-Fi")
	}
}

func TestManualAutoSwitchUsesFreshNetwork(t *testing.T) {
	fixture := newEngineFixture(t, `{"auto_switch":{"enabled":false,"rules":[{"match":"ssid","value":"Home","action":"use","profile":"本机"}]},"profiles":[{"name":"本机","server":"127.0.0.1:7890"}]}`)
	if err := fixture.engine.ApplyAutoSwitchWithNetwork(NetworkInfo{Ssids: []string{"Home"}, WifiNameRead: true}); err != nil {
		t.Fatal(err)
	}
	if fixture.engine.Status().State != statusOn || fixture.engine.Config().AutoSwitch.Enabled {
		t.Fatal("手动应用没有按刚读取的网络执行，或意外开启了持续自动切换")
	}
}

func TestTrayIconConfig(t *testing.T) {
	for _, mode := range []string{"brand", "toggle"} {
		config, err := parseConfig(`{"tray_icon":"` + mode + `"}`)
		if err != nil || config.TrayIcon != mode {
			t.Fatalf("图标样式 %s 未正确保存：%v", mode, err)
		}
	}
	config, err := parseConfig(`{}`)
	if err != nil || config.TrayIcon != "brand" {
		t.Fatal("旧配置缺少图标字段时应使用应用图标")
	}
	if _, err := parseConfig(`{"tray_icon":"invalid"}`); err == nil {
		t.Fatal("未知图标样式没有拒绝保存")
	}
}
