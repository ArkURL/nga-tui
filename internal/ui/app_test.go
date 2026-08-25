package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/config"
	"github.com/ArkURL/nga-tui/internal/model"
)

func j() tea.Msg   { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}} }
func ent() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }
func esc() tea.Msg { return tea.KeyMsg{Type: tea.KeyEsc} }

// dispatch 模拟 bubbletea 的消息循环，但不执行会触发真实网络请求或长时间阻塞的命令。
// 导航消息会回喂给 App（写入状态/切屏）；数据加载结果由测试手动喂入；
// 光标闪烁等定时器命令在超时后丢弃。
func dispatch(app *App, t *testing.T, msg tea.Msg) {
	_, cmd := app.Update(msg)
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() {
		done <- cmd()
	}()
	select {
	case next := <-done:
		if next == nil {
			return
		}
		if _, ok := next.(tea.QuitMsg); ok {
			return
		}
		if nav, ok := next.(NavigateMsg); ok {
			_, _ = app.Update(nav)
		}
	case <-time.After(100 * time.Millisecond):
		// 定时器/闪烁类 cmd，丢弃
	}
}

func TestAppForumToListToReader(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)

	// 分类加载完成
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	if len(app.forum.selIdx) != 3 {
		t.Fatalf("期望 3 个可选项，得到 %d", len(app.forum.selIdx))
	}

	// j 下移 2 次 → 第 3 个版面
	for i := 0; i < 2; i++ {
		m, _ := app.forum.Update(j())
		app.forum = m
	}
	if app.forum.cursor != 2 {
		t.Fatalf("2 次 j 后 cursor 应为 2，得到 %d", app.forum.cursor)
	}

	// Enter 进入帖子列表（走 App 路由 + 导航处理）
	dispatch(app, t, ent())
	if app.screen != ScreenThreadList {
		t.Fatalf("App 屏幕应为 ThreadList，得到 %v", app.screen)
	}
	if app.state.CurrentForum == nil || app.state.CurrentForum.FID != "3" {
		t.Fatalf("CurrentForum 应为 fid=3，得到 %+v", app.state.CurrentForum)
	}

	// 帖子列表加载完成
	app.list, _ = app.list.Update(threadsLoadedMsg{res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 10, Subject: "主题1"}},
		Page:    1,
		Pages:   3,
	}, err: nil})

	// Enter 打开帖子 → 阅读视图
	dispatch(app, t, ent())
	if app.screen != ScreenReader {
		t.Fatalf("期望进入阅读视图，得到 %v", app.screen)
	}
	if app.state.CurrentThread == nil || app.state.CurrentThread.TID != 10 {
		t.Fatalf("CurrentThread 应为 tid=10，得到 %+v", app.state.CurrentThread)
	}
}

func TestAppBackNavigation(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	app.list.st = app.state

	// forum → list
	dispatch(app, t, ent())
	if app.screen != ScreenThreadList {
		t.Fatalf("应进入帖子列表，得到 %v", app.screen)
	}
	app.list, _ = app.list.Update(threadsLoadedMsg{res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 1}}, Page: 1, Pages: 1,
	}, err: nil})

	// q 返回 forum
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if app.screen != ScreenForum {
		t.Fatalf("q 应返回版面，得到 %v", app.screen)
	}
}

func TestAppHelpToggle(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})

	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if app.screen != ScreenHelp {
		t.Fatalf("? 应打开帮助，screen=%v", app.screen)
	}
	if app.lastNonHelp != ScreenForum {
		t.Fatalf("lastNonHelp 应为 forum，得到 %v", app.lastNonHelp)
	}
	dispatch(app, t, esc())
	if app.screen != ScreenForum {
		t.Fatalf("Esc 应关闭帮助，screen=%v", app.screen)
	}
}

func TestAppLoginLogout(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)

	// L → 登录视图
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if app.screen != ScreenLogin {
		t.Fatalf("L 应进入登录视图，screen=%v", app.screen)
	}

	// 模拟登录成功
	sess := &api.Session{UID: "42", Cookies: map[string]string{
		"ngaPassportUid": "42", "ngaPassportCid": "token",
	}}
	app.onLoginSuccess(sess)
	if !app.state.LoggedIn {
		t.Fatal("登录后 LoggedIn 应为 true")
	}
	if !app.state.Client.LoggedIn() {
		t.Fatal("客户端应带上 cookie")
	}

	// 返回版面
	dispatch(app, t, esc())
	if app.screen != ScreenForum {
		t.Fatalf("Esc 应返回版面，得到 %v", app.screen)
	}

	// 再按 L 进入登录视图（不再静默登出），按 X 登出
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if app.screen != ScreenLogin {
		t.Fatalf("L 应进入登录视图，得到 %v", app.screen)
	}
	if !app.state.LoggedIn {
		t.Fatal("L 不应直接登出，应保持登录")
	}
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if app.state.LoggedIn {
		t.Fatal("登录视图按 X 后 LoggedIn 应为 false")
	}
	if app.state.Client.LoggedIn() {
		t.Fatal("登出后客户端不应有 cookie")
	}
}

func TestAppSearchThreadNavigation(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	app.list.st = app.state
	app.search.st = app.state

	// 进入帖子列表
	dispatch(app, t, ent())

	// / → 搜索（帖子模式）
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if app.screen != ScreenSearch {
		t.Fatalf("/ 应进入搜索，screen=%v", app.screen)
	}
	if app.search.scope != searchScopeThread {
		t.Fatalf("帖子列表 / 应为版内搜帖，得到 %v", app.search.scope)
	}
}

func TestAppForumSearch(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	app.search.st = app.state

	// 版面视图 / → 搜索版面
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if app.screen != ScreenSearch || app.search.scope != searchScopeForum {
		t.Fatalf("版面 / 应为搜版面，screen=%v scope=%v", app.screen, app.search.scope)
	}

	// 输入关键字并回车
	app.search.input.SetValue("版面3")
	dispatch(app, t, ent())
	if !app.search.showResults || len(app.search.results) != 1 {
		t.Fatalf("应匹配 1 个版面，得到 %d", len(app.search.results))
	}

	// 回车进入匹配的版面
	dispatch(app, t, ent())
	if app.screen != ScreenThreadList || app.state.CurrentForum == nil || app.state.CurrentForum.FID != "3" {
		t.Fatalf("应进入 fid=3 帖子列表，screen=%v forum=%+v", app.screen, app.state.CurrentForum)
	}
}

func TestAppLoginTypingLStaysInChoice(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)

	// 进入登录视图（选择方式）
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if app.screen != ScreenLogin {
		t.Fatalf("L 应进入登录视图，得到 %v", app.screen)
	}

	// 在登录视图（选择方式）中按字母键不应触发登录切换/退出
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if app.screen != ScreenLogin {
		t.Fatalf("登录视图按 l 不应切走，得到 %v", app.screen)
	}

	// B 进入浏览器抓取模式（只检查状态，不执行抓取命令以免真正打开浏览器）
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	if app.login.mode != loginBrowser {
		t.Fatalf("B 应进入浏览器抓取模式，得到 %v", app.login.mode)
	}
	if cmd == nil {
		t.Fatal("B 应返回抓取命令")
	}
}

func TestSessionPersistRoundTrip(t *testing.T) {
	// 用临时 HOME 隔离，避免污染真实配置
	t.Setenv("HOME", t.TempDir())

	client := api.NewClient()
	cookies := map[string]string{
		"ngaPassportUid": "12345",
		"ngaPassportCid": "abcdef",
		"_ga":            "GA1.1",
	}
	client.SetCookies(cookies)
	persistAll(client.Cookies(), map[string]bool{"7": true})

	// 模拟重启：从配置读取
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.Cookies["ngaPassportUid"] != "12345" || cfg.Cookies["ngaPassportCid"] != "abcdef" {
		t.Fatalf("cookie 未持久化: %+v", cfg.Cookies)
	}
	if cfg.Cookies["_ga"] != "GA1.1" {
		t.Fatalf("应保留完整 cookie 集: %+v", cfg.Cookies)
	}
	if len(cfg.Favorites) != 1 || cfg.Favorites[0] != "7" {
		t.Fatalf("收藏未持久化: %+v", cfg.Favorites)
	}
}
