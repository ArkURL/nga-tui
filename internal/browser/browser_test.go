package browser

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestCDPCookies 启动无头 Chrome，通过 CDP 设置 NGA cookie 并读回，
// 验证 CDP 客户端与 passport 提取逻辑。
func TestCDPCookies(t *testing.T) {
	chrome, err := findChrome()
	if err != nil {
		t.Skip("未安装 Chrome，跳过")
	}
	profile, err := os.MkdirTemp("", "nga-tui-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(profile)

	cmd := exec.Command(chrome,
		"--headless=new",
		"--remote-debugging-port=0",
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		"about:blank",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 Chrome 失败: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	port, err := waitForDevToolsPort(profile, 20*time.Second)
	if err != nil {
		t.Fatalf("等待调试端口失败: %v", err)
	}
	conn, err := connectPage(port)
	if err != nil {
		t.Fatalf("连接调试端点失败: %v", err)
	}
	defer conn.Close()
	client := &cdpClient{conn: conn}

	// 通过 CDP 写入 cookie（模拟用户登录后的状态）
	for _, c := range []struct{ name, value, domain string }{
		{UidCookie, "12345", "bbs.nga.cn"},
		{CidCookie, "abcdef", ".nga.cn"},
		{"_ga", "GA1", ".nga.cn"}, // 无关 cookie
	} {
		if _, err := client.call("Network.setCookie", map[string]any{
			"name":   c.name,
			"value":  c.value,
			"domain": c.domain,
			"url":    "https://bbs.nga.cn/",
		}); err != nil {
			t.Fatalf("setCookie(%s) 失败: %v", c.name, err)
		}
	}

	cookies, err := client.getAllCookies()
	if err != nil {
		t.Fatalf("getAllCookies 失败: %v", err)
	}
	got := extractCookies(cookies)
	if got == nil {
		t.Fatalf("未提取到 passport cookie，全部: %+v", cookies)
	}
	if got[UidCookie] != "12345" || got[CidCookie] != "abcdef" {
		t.Fatalf("提取结果不对: %+v", got)
	}
	// 无关 cookie 也应保留（完整会话）
	if got["_ga"] != "GA1" {
		t.Fatalf("应保留完整 cookie 集: %+v", got)
	}
}
