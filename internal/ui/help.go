package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpModel struct {
	width  int
	height int
}

func newHelpModel() helpModel { return helpModel{} }

func (m helpModel) Init() tea.Cmd { return nil }

func (m helpModel) Update(msg tea.Msg) (helpModel, tea.Cmd) {
	if wm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = wm.Width, wm.Height
	}
	return m, nil
}

func (m helpModel) View() string {
	rows := [][2]string{
		{"j / k / ↑ / ↓", "移动光标；阅读页内按楼跳转"},
		{"Shift+J / Shift+K（阅读页）", "逐行细调滚动"},
		{"Enter / l", "进入当前项"},
		{"Esc / h / q", "返回上级（根级 q 退出）"},
		{"n / p / PgDn / PgUp", "翻页"},
		{"g / G", "跳到顶部 / 底部"},
		{"/", "搜索（版面视图搜版面，帖子视图搜帖）"},
		{"f（版面/搜索结果/帖子列表）", "收藏 / 取消收藏（含子版面）"},
		{"Tab（版面视图）", "切换「全部 / 收藏」版面"},
		{"e", "切换帖子排序（最后回复 / 发布时间）"},
		{"r", "刷新当前列表"},
		{"L", "登录 / 登出"},
		{"B（登录页）", "打开浏览器登录并自动抓取 Cookie"},
		{"M（登录页）", "手动粘贴 Cookie"},
		{"?", "显示 / 关闭本帮助"},
		{"Ctrl+C", "强制退出"},
	}
	var sb strings.Builder
	sb.WriteString("\n  " + titleStyle.Render("NGA TUI 键位帮助") + "\n\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("  %-24s %s\n", lipgloss.NewStyle().Bold(true).Render(r[0]), dimStyle.Render(r[1])))
	}
	sb.WriteString("\n  " + dimStyle.Render("按 q / Esc / ? 返回"))
	return sb.String()
}
