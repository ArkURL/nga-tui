package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/model"
)

func sampleCategories() []model.Category {
	return []model.Category{
		{
			Name: "分类A",
			Groups: []model.Group{
				{
					Name: "组1",
					Forums: []model.Forum{
						{FID: "1", Name: "版面1"},
						{FID: "2", Name: "版面2"},
						{FID: "3", Name: "版面3"},
					},
				},
			},
		},
	}
}

func TestForumMoveWithJKey(t *testing.T) {
	m := newForumModel()
	m.state = forumReady
	m.items = buildForumItems(sampleCategories())
	m.selIdx = selectableIndices(m.items)
	m.cursor = 0

	// j 下移
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Fatalf("2 次 j 后 cursor 应为 2，得到 %d", m.cursor)
	}
	// k 上移
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 1 {
		t.Fatalf("1 次 k 后 cursor 应为 1，得到 %d", m.cursor)
	}
	// 末尾循环
	m.cursor = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 0 {
		t.Fatalf("末尾 j 应循环回 0，得到 %d", m.cursor)
	}
}

func TestForumEnterProducesNavigate(t *testing.T) {
	m := newForumModel()
	m.state = forumReady
	m.items = buildForumItems(sampleCategories())
	m.selIdx = selectableIndices(m.items)
	m.cursor = 1 // 版面2

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter 应返回导航 cmd")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("期望 NavigateMsg，得到 %T", msg)
	}
	f, ok := nav.Payload.(model.Forum)
	if !ok || f.FID != "2" {
		t.Fatalf("期望进入 fid=2，得到 %+v", nav.Payload)
	}
}

func TestForumBackOnEmpty(t *testing.T) {
	// 没有可选项时按 Enter 不应崩溃
	m := newForumModel()
	m.state = forumLoading
	m.items = nil
	m.selIdx = nil
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("loading 状态 Enter 不应导航")
	}
	_ = m
}

func TestBuildFavItems(t *testing.T) {
	favs := map[string]model.BoardRef{"2": {FID: "2", Name: "版面2"}}
	items := buildFavItems(favs)
	if len(items) != 1 || items[0].forum.FID != "2" {
		t.Fatalf("收藏列表应为 fid=2，得到 %+v", items)
	}
	if len(buildFavItems(map[string]model.BoardRef{})) != 0 {
		t.Fatal("无收藏应为空")
	}
}

func TestForumFavoriteToggle(t *testing.T) {
	m := newForumModel()
	m.st = NewState(api.NewClient())
	m.state = forumReady
	m.st.Categories = sampleCategories()
	m.items = buildForumItems(sampleCategories())
	m.selIdx = selectableIndices(m.items)
	m.cursor = 1 // 版面2 (fid=2)

	// f 收藏
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if _, ok := m.st.Favorites["2"]; !ok {
		t.Fatal("f 应收藏 fid=2")
	}
	// 再按 f 取消收藏
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if _, ok := m.st.Favorites["2"]; ok {
		t.Fatal("再次 f 应取消收藏 fid=2")
	}
}

func TestForumToggleFilter(t *testing.T) {
	m := newForumModel()
	m.st = NewState(api.NewClient())
	m.state = forumReady
	m.st.Categories = sampleCategories()
	m.st.Favorites = map[string]model.BoardRef{
		"1": {FID: "1", Name: "版面1"},
		"3": {FID: "3", Name: "版面3"},
	}
	m.rebuild()
	if m.favOnly {
		t.Fatal("初始应为全部模式")
	}
	if len(m.items) != 1+1+3 { // 分类头+组头+3版面
		t.Fatalf("全部模式应有 5 项，得到 %d", len(m.items))
	}

	// Tab 切到收藏
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.favOnly {
		t.Fatal("Tab 应切到收藏模式")
	}
	if len(m.items) != 2 || m.items[0].forum.FID != "1" || m.items[1].forum.FID != "3" {
		t.Fatalf("收藏模式应有 fid=1,3，得到 %+v", m.items)
	}

	// 再按 Tab 切回全部
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.favOnly {
		t.Fatal("再次 Tab 应切回全部模式")
	}
}

func TestSearchResultsFavorite(t *testing.T) {
	m := newSearchModel()
	m.st = NewState(api.NewClient())
	m.scope = searchScopeForum
	m.showResults = true
	m.results = []model.Forum{{FID: "2", Name: "版面2"}}
	m.cursor = 0

	// 搜索结果中按 f 收藏
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if _, ok := m.st.Favorites["2"]; !ok {
		t.Fatal("搜索结果的 f 应收藏 fid=2")
	}
	// 再按 f 取消收藏
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if _, ok := m.st.Favorites["2"]; ok {
		t.Fatal("再次 f 应取消收藏 fid=2")
	}
}

func TestAppStartupFavOnly(t *testing.T) {
	// 有收藏：启动进入收藏视图
	app := NewApp(api.NewClient(), false, []model.BoardRef{{FID: "7"}})
	if !app.forum.favOnly {
		t.Fatal("有收藏时启动应直接显示收藏视图")
	}
	// 无收藏：启动进入全部版面
	app2 := NewApp(api.NewClient(), false, nil)
	if app2.forum.favOnly {
		t.Fatal("无收藏时启动应显示全部版面")
	}
}

func TestForumFavoriteShowsStatus(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	app.forum.st = app.state
	app.forum.state = forumReady
	app.forum.st.Categories = sampleCategories()
	app.forum.items = buildForumItems(sampleCategories())
	app.forum.selIdx = selectableIndices(app.forum.items)
	app.forum.cursor = 1 // 版面2

	_, cmd := app.forum.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	msg := cmd()
	sm, ok := msg.(statusMsg)
	if !ok || !strings.Contains(sm.text, "已收藏") || !strings.Contains(sm.text, "版面2") {
		t.Fatalf("期望「已收藏」状态提示，得到 %+v", msg)
	}
	// 再按一次 → 取消收藏
	_, cmd2 := app.forum.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	sm2, ok := cmd2().(statusMsg)
	if !ok || !strings.Contains(sm2.text, "已取消") {
		t.Fatalf("期望「已取消收藏」状态提示，得到 %+v", cmd2())
	}
}

func TestStatusClearSequence(t *testing.T) {
	app := NewApp(api.NewClient(), false, nil)
	_, _ = app.Update(statusMsg{text: "新提示"})
	if app.status != "新提示" {
		t.Fatalf("statusMsg 应设置状态，得到 %q", app.status)
	}
	// 过期清除（旧 seq）不应生效
	_, _ = app.Update(statusClearMsg{seq: app.statusSeq - 1})
	if app.status != "新提示" {
		t.Fatalf("过期清除不应生效，得到 %q", app.status)
	}
	// 匹配 seq 的清除应生效
	_, _ = app.Update(statusClearMsg{seq: app.statusSeq})
	if app.status != "" {
		t.Fatalf("匹配清除应清空，得到 %q", app.status)
	}
}
