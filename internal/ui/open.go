package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// openURLFunc 实际调用系统命令打开链接，测试时可替换。
var openURLFunc = openURL

// threadURL 生成 NGA 网页版帖子地址（page > 1 时带上页码）。
func threadURL(tid, page int) string {
	u := fmt.Sprintf("https://bbs.nga.cn/read.php?tid=%d", tid)
	if page > 1 {
		u += fmt.Sprintf("&page=%d", page)
	}
	return u
}

// openURL 用系统默认浏览器打开链接，不等待浏览器退出。
func openURL(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}

// openThreadCmd 在浏览器中打开帖子，并通过底栏提示结果。
func openThreadCmd(tid, page int) tea.Cmd {
	if tid <= 0 {
		return nil
	}
	u := threadURL(tid, page)
	return func() tea.Msg {
		if err := openURLFunc(u); err != nil {
			return statusMsg{text: "打开浏览器失败: " + err.Error()}
		}
		return statusMsg{text: "已在浏览器打开帖子"}
	}
}
