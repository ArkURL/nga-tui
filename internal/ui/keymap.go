package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap 定义全局键位。各视图只订阅自己关心的绑定。
type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Enter       key.Binding
	Back        key.Binding
	Search      key.Binding
	Goto        key.Binding
	NextPg      key.Binding
	PrevPg      key.Binding
	Top         key.Binding
	Bottom      key.Binding
	ScrollUp    key.Binding
	ScrollDown  key.Binding
	Refresh     key.Binding
	Sort        key.Binding
	ToggleSubs  key.Binding
	Favorite    key.Binding
	ToggleForum key.Binding
	Login       key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:          key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "上移")),
		Down:        key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "下移")),
		Enter:       key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("Enter", "进入")),
		Back:        key.NewBinding(key.WithKeys("esc", "h", "q"), key.WithHelp("Esc/h/q", "返回")),
		Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "搜索")),
		Goto:        key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "直达版面（粘贴链接/fid/stid）")),
		NextPg:      key.NewBinding(key.WithKeys("n", "pgdown", "ctrlf"), key.WithHelp("n/PgDn", "下一页")),
		PrevPg:      key.NewBinding(key.WithKeys("p", "pgup", "ctrlb"), key.WithHelp("p/PgUp", "上一页")),
		Top:         key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "顶部")),
		Bottom:      key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "底部")),
		ScrollUp:    key.NewBinding(key.WithKeys("K"), key.WithHelp("Shift+K", "阅读页向上细调一行")),
		ScrollDown:  key.NewBinding(key.WithKeys("J"), key.WithHelp("Shift+J", "阅读页向下细调一行")),
		Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "刷新")),
		Sort:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "切换排序")),
		ToggleSubs:  key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "子版面/帖子 视图切换")),
		Favorite:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "收藏/取消收藏")),
		ToggleForum: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "切换 全部/收藏 版面")),
		Login:       key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "登录/登出")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "帮助")),
		Quit:        key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "强制退出")),
	}
}
