package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/model"
)

func TestListDrillDownIntoSubforum(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state
	app.state.CurrentForum = &model.Forum{FID: "428", Name: "手游综合"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "428", key: "", res: &api.ThreadListResult{
		SubForums: []model.SubForum{
			{ID: "863", Name: "手机游戏快讯"},
			{ID: "29182350", Name: "评测/安利", IsCollection: true},
		},
		Threads: []model.Thread{{TID: 1, Subject: "帖子1"}},
		Page:    1, Pages: 1,
	}, err: nil})
	app.list.cursor = 1 // 选中合集

	_, cmd := app.list.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter 应返回导航 cmd")
	}
	if len(app.state.BoardStack) != 1 || app.state.BoardStack[0].FID != "428" {
		t.Fatalf("BoardStack 应压入父版面 [428]，得到 %+v", app.state.BoardStack)
	}
	// 处理导航：CurrentForum 变为合集
	app.Update(cmd())
	if app.state.CurrentForum == nil || app.state.CurrentForum.STID != "29182350" ||
		app.state.CurrentForum.Name != "评测/安利" {
		t.Fatalf("CurrentForum 应为合集，得到 %+v", app.state.CurrentForum)
	}
}

func TestListBackPopsBoardStack(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state
	// 已钻取：栈里有父版面，当前是子版面
	app.state.BoardStack = []model.Forum{{FID: "428", Name: "手游综合"}}
	app.state.CurrentForum = &model.Forum{STID: "29182350", Name: "评测/安利"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "428", stid: "29182350", key: "", res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 1}},
	}, err: nil})

	m, cmd := app.list.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Back 应返回加载 cmd")
	}
	if len(app.state.BoardStack) != 0 {
		t.Fatalf("弹栈后 BoardStack 应为空，得到 %+v", app.state.BoardStack)
	}
	if app.state.CurrentForum == nil || app.state.CurrentForum.FID != "428" {
		t.Fatalf("CurrentForum 应恢复为父版面，得到 %+v", app.state.CurrentForum)
	}
	if m.state != listLoading {
		t.Fatalf("弹栈后应重新加载，得到 %v", m.state)
	}
}

func TestListStaleStidDiscarded(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state
	app.state.CurrentForum = &model.Forum{FID: "428", STID: "29182350", Name: "评测/安利"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "428", stid: "29182350", key: "", res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 1, Subject: "合集帖子"}},
	}, err: nil})
	if app.list.state != listReady {
		t.Fatalf("匹配响应应生效，得到 %v", app.list.state)
	}
	// 过期响应（stid 不匹配）应被丢弃
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "428", stid: "OTHER", key: "", res: &api.ThreadListResult{
		Threads: []model.Thread{{TID: 9, Subject: "过期帖子"}},
	}, err: nil})
	if len(app.list.st.Threads) != 1 || app.list.st.Threads[0].Subject != "合集帖子" {
		t.Fatalf("过期响应应被丢弃，得到 %+v", app.list.st.Threads)
	}
}

func TestListFavoriteSubforum(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state
	app.state.CurrentForum = &model.Forum{FID: "428", Name: "手游综合"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "428", key: "", res: &api.ThreadListResult{
		SubForums: []model.SubForum{{ID: "29182350", Name: "评测/安利", IsCollection: true}},
		Threads:   []model.Thread{{TID: 1}},
	}, err: nil})
	app.list.cursor = 0 // 光标在子版面

	_, _ = app.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	ref, ok := app.state.Favorites["29182350"]
	if !ok || ref.STID != "29182350" || ref.Name != "评测/安利" {
		t.Fatalf("应收藏合集，得到 %+v", ref)
	}
}

func TestFavOnlyShowsSubforum(t *testing.T) {
	app := NewApp(api.NewClient(), false, []model.BoardRef{{STID: "29182350", Name: "评测/安利"}})
	app.forum, _ = app.forum.Update(categoriesLoadedMsg{cats: sampleCategories(), err: nil})
	if !app.forum.favOnly {
		t.Fatal("有收藏应进入收藏视图")
	}
	if len(app.forum.items) != 1 || app.forum.items[0].forum.STID != "29182350" {
		t.Fatalf("收藏视图应显示子版面收藏，得到 %+v", app.forum.items)
	}
}

func TestParseGotoBoard(t *testing.T) {
	cases := []struct {
		in        string
		fid, stid string
		wantErr   bool
	}{
		{"stid=47206901", "", "47206901", false},
		{"https://bbs.nga.cn/thread.php?stid=47206901", "", "47206901", false},
		{"thread.php?stid=47206901&page=2", "", "47206901", false},
		{"fid=7", "7", "", false},
		{"https://bbs.nga.cn/thread.php?fid=-7", "-7", "", false},
		{"7", "7", "", false},
		{"hello", "", "", true},
	}
	for _, c := range cases {
		f, err := parseGotoBoard(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("%q 应报错", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q 解析失败: %v", c.in, err)
		}
		if f.FID != c.fid || f.STID != c.stid {
			t.Fatalf("%q → FID=%q STID=%q, 期望 FID=%q STID=%q", c.in, f.FID, f.STID, c.fid, c.stid)
		}
	}
}

func TestForumGotoOpensSearch(t *testing.T) {
	m := newForumModel()
	m.state = forumReady
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("o 应返回导航 cmd")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok || nav.Screen != ScreenSearch {
		t.Fatalf("期望进入搜索视图，得到 %+v", msg)
	}
	if nav.Payload != searchScopeGoto {
		t.Fatalf("期望 goto scope，得到 %v", nav.Payload)
	}
}

func TestSearchGotoSubmitNavigates(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.search.st = app.state
	app.search.scope = searchScopeGoto
	app.search.input.SetValue("stid=47206901")
	_, cmd := app.search.submit()
	if cmd == nil {
		t.Fatal("submit 应返回导航 cmd")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok || nav.Screen != ScreenThreadList {
		t.Fatalf("期望进入帖子列表，得到 %+v", msg)
	}
	f, ok := nav.Payload.(model.Forum)
	if !ok || f.STID != "47206901" {
		t.Fatalf("期望直达合集 stid=47206901，得到 %+v", nav.Payload)
	}
}

func TestListBackfillsBoardName(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state
	// 直达时版面名只是 id
	app.state.CurrentForum = &model.Forum{STID: "47206901", Name: "47206901"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "", stid: "47206901", key: "", res: &api.ThreadListResult{
		BoardName: "[股市]技术分析",
		Threads:   []model.Thread{{TID: 1}},
	}, err: nil})
	if app.state.CurrentForum.Name != "[股市]技术分析" {
		t.Fatalf("版面名应回填为真实名称，得到 %q", app.state.CurrentForum.Name)
	}
}

func TestFilterSelfSubForums(t *testing.T) {
	subs := []model.SubForum{
		{ID: "863", Name: "手机游戏快讯"},
		{ID: "47206901", Name: "[股市]技术分析", IsCollection: true},
	}
	cur := &model.Forum{FID: "510567", STID: "47206901"}
	out := filterSelfSubForums(subs, cur)
	if len(out) != 1 || out[0].ID != "863" {
		t.Fatalf("应过滤掉指向自身的合集，得到 %+v", out)
	}
}

func TestGotoBoardFavoriteUsesRealName(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.list.st = app.state
	// 直达合集 → 加载后名称回填 → 列表内 f 收藏
	app.state.CurrentForum = &model.Forum{STID: "47206901", Name: "47206901"}
	app.list, _ = app.list.Update(threadsLoadedMsg{fid: "", stid: "47206901", key: "", res: &api.ThreadListResult{
		BoardName: "[股市]技术分析",
		Threads:   []model.Thread{{TID: 1}},
	}, err: nil})
	_, _ = app.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	ref, ok := app.state.Favorites["47206901"]
	if !ok || ref.STID != "47206901" || ref.Name != "[股市]技术分析" {
		t.Fatalf("收藏应使用回填后的真实名称，得到 %+v", ref)
	}
}
