package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/model"
)

func newTestState() *State {
	st := NewState(api.NewClient())
	st.CurrentForum = &model.Forum{FID: "7", Name: "测试版面"}
	st.Threads = []model.Thread{{TID: 1, Subject: "a"}, {TID: 2, Subject: "b"}}
	st.ListPage = 1
	st.ListPages = 5
	st.ListOrderBy = "lastpostdesc"
	st.CurrentThread = &model.Thread{TID: 1, Subject: "测试帖"}
	st.Replies = []model.Reply{{PID: 0, Lou: 0, Content: "主楼"}, {PID: 9, Lou: 1, Content: "回复"}}
	st.ReadPage = 1
	st.ReadPages = 3
	return st
}

func TestThreadListPageNav(t *testing.T) {
	m := newThreadListModel()
	m.st = newTestState()
	m.state = listReady

	// n → 下一页
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil || m.st.ListPage != 2 {
		t.Fatalf("n 应翻到第 2 页并触发加载，当前页=%d", m.st.ListPage)
	}
	// p → 上一页
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil || m.st.ListPage != 1 {
		t.Fatalf("p 应回到第 1 页，当前页=%d", m.st.ListPage)
	}
	// 首页再按 p 不越界
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd != nil || m.st.ListPage != 1 {
		t.Fatalf("首页 p 不应越界，当前页=%d", m.st.ListPage)
	}
}

func TestThreadListSortToggle(t *testing.T) {
	m := newThreadListModel()
	m.st = newTestState()
	m.state = listReady

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil || m.st.ListOrderBy != "postdatedesc" {
		t.Fatalf("e 应切换排序到 postdatedesc，得到 %s", m.st.ListOrderBy)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil || m.st.ListOrderBy != "lastpostdesc" {
		t.Fatalf("再次 e 应切回 lastpostdesc，得到 %s", m.st.ListOrderBy)
	}
}

func TestThreadListBackAndSearch(t *testing.T) {
	m := newThreadListModel()
	m.st = newTestState()
	m.state = listReady

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if msg := cmd(); msg.(NavigateMsg).Screen != ScreenForum {
		t.Fatalf("q 应返回版面视图，得到 %v", msg)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if msg := cmd(); msg.(NavigateMsg).Screen != ScreenSearch {
		t.Fatalf("/ 应进入搜索视图，得到 %v", msg)
	}
}

func TestReaderPageNav(t *testing.T) {
	m := newReaderModel()
	m.st = newTestState()
	m.state = readerReady

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil || m.st.ReadPage != 2 {
		t.Fatalf("n 应翻到阅读第 2 页，当前=%d", m.st.ReadPage)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil || m.st.ReadPage != 1 {
		t.Fatalf("p 应回到第 1 页，当前=%d", m.st.ReadPage)
	}
}

func TestReaderBack(t *testing.T) {
	m := newReaderModel()
	m.st = newTestState()
	m.state = readerReady

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if msg := cmd(); msg.(NavigateMsg).Screen != ScreenThreadList {
		t.Fatalf("q 应返回帖子列表，得到 %v", msg)
	}
}

func TestSplitRepeatedKey(t *testing.T) {
	// 长按 j：多 rune 应拆成单键
	burst := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j', 'j', 'j'}}
	keys := splitRepeatedKey(burst)
	if len(keys) != 3 {
		t.Fatalf("期望拆成 3 个键，得到 %d", len(keys))
	}
	for _, k := range keys {
		if len(k.Runes) != 1 || k.Runes[0] != 'j' {
			t.Fatalf("拆分结果不对: %+v", k.Runes)
		}
	}

	// 不同字符（IME 输入）保持原样
	ime := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'你', '好'}}
	if len(splitRepeatedKey(ime)) != 1 {
		t.Fatal("不同字符不应拆分")
	}

	// 普通单键保持原样
	single := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	if len(splitRepeatedKey(single)) != 1 {
		t.Fatal("单键不应拆分")
	}
}

func TestIsLoginError(t *testing.T) {
	if !isLoginError(fmt.Errorf("NGA 返回错误: 2048:尚未登录")) {
		t.Fatal("2048 应识别为登录错误")
	}
	if !isLoginError(fmt.Errorf("NGA 返回错误: 2048:必须登录才能使用此功能")) {
		t.Fatal("必须登录 应识别为登录错误")
	}
	if isLoginError(fmt.Errorf("HTTP 403: 该版面需要登录")) {
		t.Fatal("HTTP 403 不应误判为登录错误（那是版面权限）")
	}
	if isLoginError(nil) {
		t.Fatal("nil 不应是登录错误")
	}
}
