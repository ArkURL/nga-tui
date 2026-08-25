package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseCookies(t *testing.T) {
	s := "ngaPassportUid=12345; ngaPassportCid=abcdef123; _ga=GA1.1"
	c, err := parseCookies(s)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if c["ngaPassportUid"] != "12345" || c["ngaPassportCid"] != "abcdef123" {
		t.Fatalf("解析结果不对: %+v", c)
	}
}

func TestParseCookiesMissing(t *testing.T) {
	if _, err := parseCookies("foo=bar"); err == nil {
		t.Fatal("缺少 passport cookie 应报错")
	}
}

func TestFilterForums(t *testing.T) {
	cats := sampleCategories() // 版面1/2/3
	res := filterForums(cats, "版面2")
	if len(res) != 1 || res[0].FID != "2" {
		t.Fatalf("过滤结果不对: %+v", res)
	}
	if len(filterForums(cats, "不存在的")) != 0 {
		t.Fatal("无匹配应返回空")
	}
}

func TestLoginChoiceBrowserCapture(t *testing.T) {
	m := newLoginModel()
	m.mode = loginChoice

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	if m.mode != loginBrowser || !m.capturing {
		t.Fatalf("按 B 应进入浏览器抓取模式，mode=%v capturing=%v", m.mode, m.capturing)
	}
	if cmd == nil {
		t.Fatal("按 B 应返回抓取命令")
	}
	if m.err != nil {
		t.Fatalf("不应有错误: %v", m.err)
	}
}

func TestLoginChoiceManualMode(t *testing.T) {
	m := newLoginModel()
	m.mode = loginChoice

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}})
	if m.mode != loginManual {
		t.Fatalf("按 M 应进入手动粘贴模式，mode=%v", m.mode)
	}
	if cmd == nil {
		t.Fatal("按 M 应聚焦输入框")
	}
	// 手动模式下输入含 q 和 L 的 cookie 不应触发任何导航
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if m.manual.Value() != "nqL" {
		t.Fatalf("手动粘贴框应正常输入，得到 %q", m.manual.Value())
	}
	if m.mode != loginManual {
		t.Fatalf("输入不应切换模式，mode=%v", m.mode)
	}
}

func TestLoginEscReturns(t *testing.T) {
	m := newLoginModel()
	m.mode = loginChoice

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc 应返回版面")
	}
	nav, ok := cmd().(NavigateMsg)
	if !ok || nav.Screen != ScreenForum {
		t.Fatalf("Esc 应导航到版面，得到 %v", nav)
	}
}

func TestSearchTypeQStaysInInput(t *testing.T) {
	m := newSearchModel()
	m.st = &State{}
	m.scope = searchScopeThread
	m.input.Focus()

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.input.Value() != "q" {
		t.Fatalf("q 应被输入到搜索框，得到 %q", m.input.Value())
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(NavigateMsg); ok {
				t.Fatalf("输入 q 不应导航返回")
			}
		}
	}
}
