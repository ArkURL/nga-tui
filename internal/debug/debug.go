// Package debug 提供写入 ~/.config/nga-tui/debug.log 的诊断日志，
// 用于排查登录会话丢失等问题。
package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var mu sync.Mutex

// Logf 追加一行带时间戳的日志；目录不存在时自动创建，失败时静默忽略。
func Logf(format string, args ...any) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".config", "nga-tui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	f, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf("%s ", time.Now().Format("2006-01-02 15:04:05"))
	if len(args) == 0 {
		line += format
	} else {
		line += fmt.Sprintf(format, args...)
	}
	f.WriteString(line + "\n")
}
