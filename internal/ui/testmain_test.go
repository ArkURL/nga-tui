package ui

import (
	"fmt"
	"os"
	"testing"
)

// TestMain 把测试进程的 HOME 指向临时目录，隔离配置写入，
// 防止单元测试覆盖真实的 ~/.config/nga-tui/config.json（登录 Cookie）。
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "nga-tui-test-home-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "创建临时 HOME 失败:", err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
