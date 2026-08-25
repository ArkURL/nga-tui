// NGA TUI 客户端入口。
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/config"
	"github.com/ArkURL/nga-tui/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}

	client := api.NewClient()
	loggedIn := false
	if len(cfg.Cookies) > 0 {
		client.SetCookies(cfg.Cookies)
		ok, err := client.CheckLogin()
		switch {
		case err != nil:
			// 网络异常无法确认：乐观保留登录态（请求实际失败时会降级提示）
			loggedIn = true
		case ok:
			loggedIn = true
		default:
			// 诊断信息：cookie 存在但会话验证为未登录
			fmt.Fprintf(os.Stderr, "[nga-tui] 检测到保存的登录 Cookie 但会话已失效（可能是过期或被拒绝），按 L 可重新登录\n")
		}
	}

	app := ui.NewApp(client, loggedIn, cfg.Favorites)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("程序异常退出: %v\n", err)
	}
}
