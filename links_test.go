package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseLink(t *testing.T) {
	for link, want := range map[string]LinkRequest{
		"proxi://on":                                   {Command: "on"},
		"Proxi://OFF/":                                 {Command: "off"},
		"proxi:toggle":                                 {Command: "toggle"},
		"proxi://use?name=%E5%85%AC%E5%8F%B8":          {Command: "use", Argument: "公司"},
		"proxi://use/公司代理":                             {Command: "use", Argument: "公司代理"},
		"proxi://share":                                {Command: "share"},
		"proxi://share/on":                             {Command: "share", Argument: "on"},
		"proxi://lan?state=false":                      {Command: "share", Argument: "off"},
		"proxi://settings":                             {Command: "settings"},
		"proxi://settings?page=about":                  {Command: "settings", Argument: "about"},
		"proxi://update":                               {Command: "update"},
		"proxi://update?install=0123abcdef":            {Command: "update", Argument: "0123abcdef"},
		"proxi://update?install=../settings":           {Command: "update"},
		"proxi://diagnose":                             {Command: "diagnose", Argument: "\x00" + diagnosePc},
		"proxi://diagnose?url=youtube.com&from=device": {Command: "diagnose", Argument: "youtube.com\x00" + diagnoseDevice},
	} {
		got, err := parseLink(link)
		if err != nil || got != want {
			t.Errorf("%s 应解析为 %+v：%+v %v", link, want, got, err)
		}
	}

	// 机场网站的一键导入：clash:// 和 proxi:// 两种写法，名字可以没有，太长时截断。
	for link, want := range map[string][2]string{
		"clash://install-config?url=https%3A%2F%2Fsub.example.com%2Fapi%3Ftoken%3Dabc&name=%E6%B5%8B%E8%AF%95%E6%9C%BA%E5%9C%BA": {"https://sub.example.com/api?token=abc", "测试机场"},
		"proxi://install-config?url=http://sub.example.com/a":                                                                    {"http://sub.example.com/a", ""},
		"proxi://import?url=https://sub.example.com/a&name=" + strings.Repeat("长", 40):                                           {"https://sub.example.com/a", strings.Repeat("长", maxProfileNameLength)},
	} {
		got, err := parseLink(link)
		if err != nil || got.Command != "import" {
			t.Errorf("%s 应是导入订阅：%+v %v", link, got, err)
			continue
		}
		var request map[string]string
		if err := json.Unmarshal([]byte(got.Argument), &request); err != nil || request["url"] != want[0] || request["name"] != want[1] {
			t.Errorf("%s 的订阅地址和名字不对：%s", link, got.Argument)
		}
	}

	for link, problem := range map[string]string{
		"proxi://use":                                    "配置名",
		"proxi://settings?page=../x":                     "设置页",
		"proxi://unknown":                                "不认识",
		"clash://other?url=https://sub.example.com":      "不认识",
		"clash://install-config":                         "订阅地址",
		"clash://install-config?url=file:///C:/sub.yaml": "订阅地址",
		"proxi://install-config?url=javascript:alert":    "订阅地址",
		"proxi://diagnose?url=ftp://example.com":         "",
		"http://example.com":                             "不认识",
	} {
		if _, err := parseLink(link); err == nil || !strings.Contains(err.Error(), problem) {
			t.Errorf("%s 应报错（%s）：%v", link, problem, err)
		}
	}

	for text, want := range map[string]bool{"proxi://on": true, "CLASH://install-config": true, " proxi:toggle": true, "on": false, "--autostart": false, "http://x": false} {
		if isLink(text) != want {
			t.Errorf("isLink(%q) 应为 %v", text, want)
		}
	}
}
