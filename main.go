// NGA TUI 客户端入口。
package main

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/config"
	"github.com/ArkURL/nga-tui/internal/debug"
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
			debug.Logf("启动: 登录检查失败（网络），乐观视为已登录: %v", err)
			loggedIn = true
		case ok:
			loggedIn = true
		default:
			debug.Logf("启动: 检测到 %d 个已保存 cookie（%v）但会话失效（2048）", len(cfg.Cookies), cookieNameList(cfg.Cookies))
		}
	} else {
		debug.Logf("启动: 无已保存 cookie")
	}

	app := ui.NewApp(client, loggedIn, cfg.Favorites)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("程序异常退出: %v\n", err)
	}
}

// cookieNameList 返回 cookie 键名列表（不打印值）。
func cookieNameList(cookies map[string]string) []string {
	out := make([]string, 0, len(cookies))
	for k := range cookies {
		out = append(out, k)
	}
	return out
}
