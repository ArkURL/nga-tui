package ui

import (
	"strings"
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
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "3", key: "", res: &api.ThreadListResult{
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
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "1", key: "", res: &api.ThreadListResult{
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

	// 再按 L 进入登录视图（不再静默登出），X 后需 Y 确认登出
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if app.screen != ScreenLogin {
		t.Fatalf("L 应进入登录视图，得到 %v", app.screen)
	}
	if !app.state.LoggedIn {
		t.Fatal("L 不应直接登出，应保持登录")
	}
	// 按 X 进入确认，此时不应登出
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if !app.state.LoggedIn || !app.login.confirmLogout {
		t.Fatal("按 X 应进入确认状态，不应立即登出")
	}
	// 按其他键取消
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if app.login.confirmLogout {
		t.Fatal("其他键应取消确认")
	}
	if !app.state.LoggedIn {
		t.Fatal("取消确认后应仍保持登录")
	}
	// 重新 X + Y 确认登出
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	dispatch(app, t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	if app.state.LoggedIn {
		t.Fatal("登录视图 X+Y 后 LoggedIn 应为 false")
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
	persistAll(client.Cookies(), map[string]model.BoardRef{"7": {FID: "7", Name: "版7"}})

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
	if len(cfg.Favorites) != 1 || cfg.Favorites[0].Key() != "7" {
		t.Fatalf("收藏未持久化: %+v", cfg.Favorites)
	}
}

func TestBackFromReaderKeepsListState(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	app.list.st = app.state

	// 进入帖子列表（版面入口 → 加载）
	dispatch(app, t, ent())
	if app.screen != ScreenThreadList {
		t.Fatalf("应进入帖子列表，得到 %v", app.screen)
	}
	// 手动喂数据并移动选中项
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "1", key: "", res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 1}, {TID: 2}, {TID: 3}},
		Page:    1, Pages: 2,
	}, err: nil})
	app.list.cursor = 1

	// 进入帖子（阅读）——选中第 2 条 (TID=2)
	dispatch(app, t, ent())
	app.reader, _ = app.reader.Update(contentLoadedMsg{tid: 2, res: &api.ThreadContentResult{
		Replies: []model.Reply{{PID: 0, Lou: 0, Content: "主楼"}},
		Page:    1, Pages: 1,
	}, err: nil})
	if app.screen != ScreenReader {
		t.Fatalf("应进入阅读视图，得到 %v", app.screen)
	}

	// 返回帖子列表
	dispatch(app, t, esc())
	if app.screen != ScreenThreadList {
		t.Fatalf("应返回帖子列表，得到 %v", app.screen)
	}
	// 关键断言：列表保持 ready、选中项不变、不触发加载
	if app.list.state != listReady {
		t.Fatalf("返回后列表应为 ready，得到 %v", app.list.state)
	}
	if app.list.cursor != 1 {
		t.Fatalf("返回后选中项应为 1，得到 %d", app.list.cursor)
	}
	if len(app.list.st.Threads) != 3 {
		t.Fatalf("列表数据不应变化，得到 %d 条", len(app.list.st.Threads))
	}
}

func TestReaderNavToNewThreadShowsLoadingNotOldContent(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.reader.st = app.state

	// 帖 A 已加载完成
	app.state.CurrentThread = &model.Thread{TID: 1}
	app.reader, _ = app.reader.Update(contentLoadedMsg{tid: 1, res: &api.ThreadContentResult{
		Replies: []model.Reply{{Lou: 0, Content: "帖子A内容"}},
	}, err: nil})
	if app.reader.state != readerReady {
		t.Fatalf("A 应已就绪，得到 %v", app.reader.state)
	}

	// 导航到帖 B：不执行返回的 cmd，仅验证导航副作用
	_, cmd := app.Update(NavigateMsg{Screen: ScreenReader, Payload: model.Thread{TID: 2}})
	if cmd == nil {
		t.Fatal("进入 B 应返回加载命令")
	}
	if app.reader.state != readerLoading {
		t.Fatalf("进入 B 后应立即 loading，得到 %v", app.reader.state)
	}
	if strings.Contains(app.reader.View(), "帖子A内容") {
		t.Fatal("loading 时不应显示 A 的内容")
	}

	// 闭环：喂 B 的响应（先设视口尺寸以便渲染）
	app.reader.width = 80
	app.reader.vp.Width = 80
	app.reader.vp.Height = 20
	app.reader, _ = app.reader.Update(contentLoadedMsg{tid: 2, res: &api.ThreadContentResult{
		Replies: []model.Reply{{Lou: 0, Content: "帖子B内容"}},
	}, err: nil})
	if app.reader.state != readerReady || !strings.Contains(app.reader.View(), "帖子B内容") {
		t.Fatal("B 加载完成应显示 B 内容")
	}
}

func TestListNavToNewBoardShowsLoadingNotOldList(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state

	// 版面 A 已加载完成
	app.state.CurrentForum = &model.Forum{FID: "1", Name: "版面1"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "1", key: "", res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 1, Subject: "旧版面帖子"}},
		Page:    1, Pages: 1,
	}, err: nil})
	if app.list.state != listReady {
		t.Fatalf("旧版面应已就绪，得到 %v", app.list.state)
	}

	// 导航到版面 B
	_, cmd := app.Update(NavigateMsg{Screen: ScreenThreadList, Payload: model.Forum{FID: "2", Name: "版面2"}})
	if cmd == nil {
		t.Fatal("切版面应返回加载命令")
	}
	if app.list.state != listLoading {
		t.Fatalf("切版面后应立即 loading，得到 %v", app.list.state)
	}
	if strings.Contains(app.list.View(), "旧版面帖子") {
		t.Fatal("loading 时不应显示旧版面的帖子")
	}

	// 闭环：喂新版面响应（先设视口尺寸以便渲染）
	app.list.width = 80
	app.list.vp.Width = 80
	app.list.vp.Height = 20
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "2", key: "", res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 9, Subject: "新版面帖子"}},
		Page:    1, Pages: 1,
	}, err: nil})
	if app.list.state != listReady || !strings.Contains(app.list.View(), "新版面帖子") {
		t.Fatal("新版面加载完成应显示新版面帖子")
	}
}

func TestForumBackKeepsState(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum.st = app.state
	// forum 首次加载完成
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	if app.forum.state != forumReady {
		t.Fatalf("forum 应就绪，得到 %v", app.forum.state)
	}

	// 导航到帖子列表
	_, _ = app.Update(NavigateMsg{Screen: ScreenThreadList, Payload: model.Forum{FID: "1"}})
	if app.screen != ScreenThreadList {
		t.Fatalf("应进入帖子列表，得到 %v", app.screen)
	}

	// 返回 forum：已就绪则保持状态，不应触发加载
	_, cmd := app.Update(NavigateMsg{Screen: ScreenForum, Payload: nil})
	if cmd != nil {
		t.Fatal("返回 forum 且已就绪时不应触发加载命令")
	}
	if app.screen != ScreenForum {
		t.Fatalf("应返回 forum，得到 %v", app.screen)
	}
	if app.forum.state != forumReady {
		t.Fatalf("返回后 forum 应保持 ready，得到 %v", app.forum.state)
	}
	if len(app.forum.items) != 5 { // 分类头+组头+3版面
		t.Fatalf("分类数据不应变化，得到 %d 项", len(app.forum.items))
	}
}

func TestPersistAllPreservesLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// 先写入带登录 cookie 的配置
	client := api.NewClient()
	client.SetCookies(map[string]string{"ngaPassportUid": "1", "ngaPassportCid": "token"})
	persistAll(client.Cookies(), map[string]model.BoardRef{})

	// 模拟访客实例（空 cookie）收藏 → 不应覆盖已保存的登录 cookie
	persistAll(map[string]string{}, map[string]model.BoardRef{"7": {FID: "7", Name: "版7"}})

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cookies["ngaPassportUid"] != "1" || cfg.Cookies["ngaPassportCid"] != "token" {
		t.Fatalf("空 cookie 写入不应覆盖登录 cookie: %+v", cfg.Cookies)
	}
	if len(cfg.Favorites) != 1 || cfg.Favorites[0].Key() != "7" {
		t.Fatalf("收藏应正常写入: %+v", cfg.Favorites)
	}
}

func TestLogoutStillClearsLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	client := api.NewClient()
	client.SetCookies(map[string]string{"ngaPassportUid": "1", "ngaPassportCid": "token"})
	persistAll(client.Cookies(), map[string]model.BoardRef{})

	// 登出仍应清空 cookie
	persistAllRaw(map[string]string{}, map[string]model.BoardRef{})
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Cookies) != 0 {
		t.Fatalf("登出应清空 cookie: %+v", cfg.Cookies)
	}
}
