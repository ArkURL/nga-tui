package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/model"
)

func TestThreadURL(t *testing.T) {
	cases := []struct {
		tid, page int
		want      string
	}{
		{123, 1, "https://bbs.nga.cn/read.php?tid=123"},
		{123, 3, "https://bbs.nga.cn/read.php?tid=123&page=3"},
	}
	for _, c := range cases {
		if got := threadURL(c.tid, c.page); got != c.want {
			t.Fatalf("threadURL(%d, %d) = %q, want %q", c.tid, c.page, got, c.want)
		}
	}
}

// withOpenURL 替换打开器，返回捕获的 URL 与恢复函数。
func withOpenURL(t *testing.T, err error) (*string, func()) {
	t.Helper()
	old := openURLFunc
	var got string
	openURLFunc = func(u string) error {
		got = u
		return err
	}
	return &got, func() { openURLFunc = old }
}

func TestReaderOpenBrowser(t *testing.T) {
	got, restore := withOpenURL(t, nil)
	defer restore()

	st := NewState(api.NewClient())
	st.CurrentThread = &model.Thread{TID: 123}
	st.ReadPage = 3
	m := readerModel{st: st}

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if cmd == nil {
		t.Fatal("按 o 应返回打开浏览器的命令")
	}
	msg := cmd()
	if want := "https://bbs.nga.cn/read.php?tid=123&page=3"; *got != want {
		t.Fatalf("打开 URL = %q, want %q", *got, want)
	}
	if _, ok := msg.(statusMsg); !ok {
		t.Fatalf("期望 statusMsg，得到 %T", msg)
	}
}

func TestThreadListOpenBrowser(t *testing.T) {
	got, restore := withOpenURL(t, nil)
	defer restore()

	st := NewState(api.NewClient())
	st.Threads = []model.Thread{{TID: 1}, {TID: 2}}
	m := threadListModel{st: st, state: listReady}
	m.cursor = 1

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if cmd == nil {
		t.Fatal("按 o 应返回打开浏览器的命令")
	}
	cmd()
	if want := "https://bbs.nga.cn/read.php?tid=2"; *got != want {
		t.Fatalf("打开 URL = %q, want %q", *got, want)
	}
}

func TestOpenBrowserFailureStatus(t *testing.T) {
	_, restore := withOpenURL(t, errors.New("no browser"))
	defer restore()

	msg := openThreadCmd(1, 1)()
	sm, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("期望 statusMsg，得到 %T", msg)
	}
	if !strings.Contains(sm.text, "no browser") {
		t.Fatalf("状态提示应包含错误原因，得到 %q", sm.text)
	}
}
