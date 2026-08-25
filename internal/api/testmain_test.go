package api

import (
	"fmt"
	"os"
	"testing"
)

// TestMain 隔离测试进程的 HOME，避免测试污染真实的
// ~/.config/nga-tui/debug.log 与 config.json。
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "nga-tui-api-test-home-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "创建临时 HOME 失败:", err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
