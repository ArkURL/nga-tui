package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// footerHeight 是 App 底栏占用的行数。
const footerHeight = 2

// km 是全局键位表。
var km = newKeyMap()

// keyMatches 判断按键是否匹配绑定。
func keyMatches(msg tea.KeyMsg, b key.Binding) bool {
	return key.Matches(msg, b)
}

// isEsc 判断是否为 Esc 键。文本输入视图用 Esc 做返回，
// 避免 q/h 这类字母键被拦截而无法输入。
func isEsc(msg tea.KeyMsg) bool {
	return msg.String() == "esc"
}

// splitRepeatedKey 把长按产生的重复字符（如 "jjjj"）拆成单个按键。
// bubbletea 会把终端送来的连续相同字符合并为一个多 rune 的 KeyMsg，
// 导致与单字符绑定匹配失败。不同字符的输入（IME 等）保持原样。
func splitRepeatedKey(msg tea.KeyMsg) []tea.KeyMsg {
	if msg.Type != tea.KeyRunes || len(msg.Runes) <= 1 {
		return []tea.KeyMsg{msg}
	}
	for _, r := range msg.Runes {
		if r != msg.Runes[0] {
			return []tea.KeyMsg{msg}
		}
	}
	out := make([]tea.KeyMsg, len(msg.Runes))
	for i, r := range msg.Runes {
		out[i] = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
	}
	return out
}

// navCmd 返回一个发送 NavigateMsg 的 tea.Cmd。
func navCmd(s Screen, payload any) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{Screen: s, Payload: payload}
	}
}

// truncateLine 按显示宽度截断一行（兼容 CJK/ANSI），超宽加省略号。
func truncateLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := ansi.StringWidthWc(s)
	if w <= width {
		return s
	}
	return ansi.TruncateWc(s, max(width-1, 0), "…")
}

// searchScope 表示搜索模式。
type searchScope int

const (
	// searchScopeForum 按名称搜索版面。
	searchScopeForum searchScope = iota
	// searchScopeThread 在当前版面内搜索帖子。
	searchScopeThread
	// searchScopeGoto 粘贴链接或 fid/stid 直达版面。
	searchScopeGoto
)

// statusMsg 是瞬态状态提示（如"已收藏「xxx」"），由视图发送、App 统一渲染到底栏。
type statusMsg struct{ text string }

// statusClearMsg 清除状态提示，携带序号以丢弃过期的清除（新的提示会递增序号）。
type statusClearMsg struct{ seq int }

// statusCmd 生成发送状态提示的命令。
func statusCmd(text string) tea.Cmd {
	return func() tea.Msg { return statusMsg{text: text} }
}

// formatTime 把 unix 时间戳格式化为 "01-02 15:04"。
func formatTime(ts int64) string {
	if ts <= 0 {
		return "?"
	}
	return time.Unix(ts, 0).Format("01-02 15:04")
}

// isLoginError 判断错误是否为"未登录/登录失效"（NGA 错误码 2048）。
func isLoginError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "2048") || strings.Contains(s, "尚未登录") || strings.Contains(s, "必须登录")
}
