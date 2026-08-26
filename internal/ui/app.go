package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/config"
	"github.com/ArkURL/nga-tui/internal/debug"
	"github.com/ArkURL/nga-tui/internal/model"
)

// App 是根 model：负责屏幕路由、共享状态和底栏。
type App struct {
	screen Screen
	state  *State

	login  loginModel
	forum  forumModel
	list   threadListModel
	reader readerModel
	search searchModel
	help   helpModel

	// lastNonHelp 记录进入帮助前的视图，用于返回。
	lastNonHelp Screen

	// status 是瞬态状态提示（如收藏成功），statusSeq 用于丢弃过期的清除。
	status    string
	statusSeq int

	width  int
	height int
}

// NewApp 创建根 model。loggedIn 表示启动时已恢复有效会话，favorites 是持久化的收藏版面。
func NewApp(client *api.Client, loggedIn bool, favorites []model.BoardRef) *App {
	a := &App{
		screen: ScreenForum,
		state:  NewState(client),
		login:  newLoginModel(),
		forum:  newForumModel(),
		list:   newThreadListModel(),
		reader: newReaderModel(),
		search: newSearchModel(),
		help:   newHelpModel(),
	}
	a.state.LoggedIn = loggedIn
	a.state.Favorites = map[string]model.BoardRef{}
	for _, ref := range favorites {
		if ref.Key() != "" {
			a.state.Favorites[ref.Key()] = ref
		}
	}
	// 已有收藏时，启动首页直接显示收藏版面
	a.forum.favOnly = len(a.state.Favorites) > 0
	a.forum.st = a.state
	a.list.st = a.state
	a.reader.st = a.state
	a.search.st = a.state
	a.login.onSuccess = a.onLoginSuccess
	a.login.onLogout = a.logout

	// 跟随 NGA 轮换会话 cookie（Set-Cookie），变化时持久化到本地
	client.SetOnCookiesChanged(func() {
		persistAll(client.Cookies(), a.state.Favorites)
	})
	return a
}

// onLoginSuccess 登录成功：写入 cookie、标记状态、持久化（保留收藏）。
func (a *App) onLoginSuccess(sess *api.Session) {
	a.state.Client.SetCookies(sess.Cookies)
	a.state.LoggedIn = true
	persistAll(a.state.Client.Cookies(), a.state.Favorites)
}

// logout 登出：清除 cookie 与配置（保留收藏）。
func (a *App) logout() {
	debug.Logf("执行登出")
	a.state.Client.SetCookies(map[string]string{})
	a.state.LoggedIn = false
	persistAllRaw(map[string]string{}, a.state.Favorites)
}

// persistAll 把当前 cookie 与收藏写入配置文件（登录态保护）。
// 若传入的 cookie 不含有效 passport，而配置里已有登录 cookie，则保留旧的，
// 防止收藏等操作（或另一个访客实例）意外清掉登录。
func persistAll(cookies map[string]string, favs map[string]model.BoardRef) {
	if !hasPassport(cookies) {
		if prev, err := config.Load(); err == nil && hasPassport(prev.Cookies) {
			cookies = prev.Cookies
		}
	}
	persistAllRaw(cookies, favs)
}

// persistAllRaw 直接写入配置（登出等明确要清空 cookie 时用）。
func persistAllRaw(cookies map[string]string, favs map[string]model.BoardRef) {
	debug.Logf("持久化配置: cookie名=%v 收藏数=%d", cookieNames(cookies), len(favs))
	cfg := &config.Config{Cookies: cookies, Favorites: favoriteList(favs)}
	_ = config.Save(cfg)
}

// hasPassport 判断 cookie 集是否包含有效的登录护照。
func hasPassport(cookies map[string]string) bool {
	return cookies["ngaPassportUid"] != "" && cookies["ngaPassportCid"] != ""
}

// cookieNames 返回 cookie 的键名列表（不打印值，避免泄露 token）。
func cookieNames(cookies map[string]string) []string {
	out := make([]string, 0, len(cookies))
	for k := range cookies {
		out = append(out, k)
	}
	return out
}

// favoriteList 把收藏 map 转为有序列表（按 Key 排序，保证确定性）。
func favoriteList(favs map[string]model.BoardRef) []model.BoardRef {
	out := make([]model.BoardRef, 0, len(favs))
	for _, ref := range favs {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

func (a *App) Init() tea.Cmd {
	a.forum.client = a.state.Client
	return a.forum.Init()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		// 新的状态提示：递增序号并安排清除
		a.statusSeq++
		seq := a.statusSeq
		a.status = msg.text
		return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return statusClearMsg{seq: seq} })

	case statusClearMsg:
		// 只清除仍属于最新提示的状态，避免旧清除抹掉新提示
		if msg.seq == a.statusSeq {
			a.status = ""
		}
		return a, nil

	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, a.route(msg)

	case NavigateMsg:
		cmd := a.handleNavigate(msg)
		// 视图切换后补发窗口尺寸，让新视图的 viewport 正确初始化
		if a.width > 0 && a.height > 0 {
			cmd = tea.Batch(cmd, a.route(tea.WindowSizeMsg{Width: a.width, Height: a.height}))
		}
		return a, cmd

	case tea.KeyMsg:
		// 长按产生的重复字符拆成单个按键逐个处理
		var cmds []tea.Cmd
		for _, k := range splitRepeatedKey(msg) {
			switch {
			case keyMatches(k, km.Quit):
				cmds = append(cmds, tea.Quit)
			case a.screen == ScreenHelp:
				if keyMatches(k, km.Back) || keyMatches(k, km.Help) || keyMatches(k, km.Enter) {
					a.screen = a.lastNonHelp
				}
			case a.screen == ScreenLogin || a.screen == ScreenSearch:
				// 文本输入视图：除 Ctrl+C 外所有按键直接路由，
				// 避免 L/?/q 等被全局拦截而无法输入
				if cmd := a.route(k); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case keyMatches(k, km.Help):
				if a.screen != ScreenHelp {
					a.lastNonHelp = a.screen
					a.screen = ScreenHelp
				}
			case keyMatches(k, km.Login):
				// 始终进入登录视图（视图内可登出/重新登录），避免误触静默登出
				a.screen = ScreenLogin
				a.login.client = a.state.Client
				cmds = append(cmds, a.login.reset())
			case a.screen == ScreenForum && keyMatches(k, km.Back):
				cmds = append(cmds, tea.Quit)
			default:
				if cmd := a.route(k); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		return a, tea.Batch(cmds...)
	}

	return a, a.route(msg)
}

// route 把消息分发给当前活动视图。
func (a *App) route(msg tea.Msg) tea.Cmd {
	switch a.screen {
	case ScreenForum:
		m, cmd := a.forum.Update(msg)
		a.forum = m
		return cmd
	case ScreenThreadList:
		m, cmd := a.list.Update(msg)
		a.list = m
		return cmd
	case ScreenReader:
		m, cmd := a.reader.Update(msg)
		a.reader = m
		return cmd
	case ScreenSearch:
		m, cmd := a.search.Update(msg)
		a.search = m
		return cmd
	case ScreenLogin:
		m, cmd := a.login.Update(msg)
		a.login = m
		return cmd
	case ScreenHelp:
		m, cmd := a.help.Update(msg)
		a.help = m
		return cmd
	}
	return nil
}

// handleNavigate 处理视图间的导航请求并初始化目标视图。
func (a *App) handleNavigate(msg NavigateMsg) tea.Cmd {
	switch msg.Screen {
	case ScreenForum:
		a.screen = ScreenForum
		// 已就绪则保持状态（分类低频变化，返回不重载）；加载在途不重复发起；
		// 首次/错误后重试
		switch a.forum.state {
		case forumLoading:
			return nil
		case forumReady:
			return nil
		default:
			return a.forum.start()
		}
	case ScreenThreadList:
		a.screen = ScreenThreadList
		if f, ok := msg.Payload.(model.Forum); ok {
			// 进入/切换版面：需要重新加载
			a.state.CurrentForum = &f
			a.state.ListSearchKey = ""
			a.state.ListPage = 1
			a.state.ListReload = true
		}
		if a.state.ListReload {
			a.state.ListReload = false
			return a.list.start()
		}
		// 从阅读页/搜索返回：保持列表状态与选中项，不刷新（按 r 手动刷新）
		return nil
	case ScreenReader:
		a.screen = ScreenReader
		if th, ok := msg.Payload.(model.Thread); ok {
			a.state.CurrentThread = &th
			a.state.ReadPage = 1
		}
		return a.reader.start()
	case ScreenSearch:
		a.screen = ScreenSearch
		if sc, ok := msg.Payload.(searchScope); ok {
			a.search.scope = sc
		}
		a.search.width = a.width
		return a.search.reset()
	case ScreenLogin:
		a.screen = ScreenLogin
		a.login.client = a.state.Client
		return a.login.reset()
	case ScreenHelp:
		a.screen = ScreenHelp
		return nil
	}
	return nil
}

func (a *App) View() string {
	var body string
	switch a.screen {
	case ScreenForum:
		body = a.forum.View()
	case ScreenThreadList:
		body = a.list.View()
	case ScreenReader:
		body = a.reader.View()
	case ScreenSearch:
		body = a.search.View()
	case ScreenLogin:
		body = a.login.View()
	case ScreenHelp:
		body = a.help.View()
	}
	return body + "\n" + a.footer()
}

// footer 渲染底栏：屏幕名 + 登录态 + 键位提示。
func (a *App) footer() string {
	left := fmt.Sprintf(" %s ", a.screen.String())
	if a.state.LoggedIn {
		left += okStyle.Render("● 已登录")
	} else {
		left += dimStyle.Render("○ 未登录")
	}
	if a.screen == ScreenThreadList {
		if a.state.ListSearchKey != "" {
			left += dimStyle.Render(" 搜索: " + a.state.ListSearchKey)
		}
		if a.state.ListPages > 0 {
			left += dimStyle.Render(fmt.Sprintf(" %d/%d 页", a.state.ListPage, a.state.ListPages))
		}
	} else if a.screen == ScreenReader {
		if a.state.ReadPages > 0 {
			left += dimStyle.Render(fmt.Sprintf(" %d/%d 页", a.state.ReadPage, a.state.ReadPages))
		}
	}
	if a.status != "" {
		left += " " + accentStyle.Render(a.status)
	}

	right := strings.Builder{}
	hints := []string{}
	if a.screen == ScreenForum {
		hints = append(hints, "Enter 进版面", "f 收藏", "o 直达", "Tab 收藏/全部", "/ 搜版面", "L 登录", "q 退出")
	} else if a.screen == ScreenThreadList {
		if a.state.ListSearchKey == "" && len(a.state.SubForums) > 0 {
			if a.list.showSubs {
				hints = append(hints, "t 帖子", "Enter 进入子版面")
			} else {
				hints = append(hints, "t 子版面", "Enter 看帖")
			}
		} else {
			hints = append(hints, "Enter 看帖")
		}
		hints = append(hints, "f 收藏", "n/p 翻页", "/ 搜帖", "e 排序", "q 返回")
	} else if a.screen == ScreenReader {
		hints = append(hints, "j/k 按楼跳转", "Shift+J/K 细调", "n/p 翻页", "q 返回")
	}
	hints = append(hints, "? 帮助")
	right.WriteString(dimStyle.Render(strings.Join(hints, "  ")))

	left = lipgloss.NewStyle().Width(a.width - lipgloss.Width(right.String())).MaxWidth(a.width).Render(left)
	return statusBarStyle.Render(left) + "\n" + right.String() + "\n"
}
