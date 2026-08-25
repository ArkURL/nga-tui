package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
)

type listState int

const (
	listLoading listState = iota
	listReady
	listError
)

type threadListModel struct {
	st     *State
	state  listState
	width  int
	height int
	sp     spinner.Model
	vp     viewport.Model
	err    error
	cursor int
}

type threadsLoadedMsg struct {
	res *api.ThreadListResult
	err error
}

func newThreadListModel() threadListModel {
	return threadListModel{
		state: listLoading,
		sp:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		vp:    viewport.New(0, 0),
	}
}

func loadThreadsCmd(st *State) tea.Cmd {
	return func() tea.Msg {
		res, err := st.Client.GetThreads(
			st.CurrentForum.FID,
			st.ListPage,
			st.ListOrderBy,
			st.ListSearchKey,
		)
		return threadsLoadedMsg{res: res, err: err}
	}
}

func (m threadListModel) Init() tea.Cmd {
	if m.st == nil || m.st.CurrentForum == nil {
		return nil
	}
	m.state = listLoading
	m.cursor = 0
	return tea.Batch(m.sp.Tick, loadThreadsCmd(m.st))
}

func (m threadListModel) Update(msg tea.Msg) (threadListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
			m.vp.Width = msg.Width
		}
		if msg.Height > footerHeight+1 {
			m.height = msg.Height
			m.vp.Height = msg.Height - footerHeight - 1
		}
		m.syncViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case threadsLoadedMsg:
		if msg.err != nil {
			m.state = listError
			m.err = msg.err
			if isLoginError(msg.err) && m.st != nil {
				m.st.LoggedIn = false
			}
			return m, nil
		}
		m.state = listReady
		m.st.Threads = msg.res.Threads
		m.st.ListPage = msg.res.Page
		m.st.ListPages = msg.res.Pages
		m.cursor = 0
		m.syncViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m threadListModel) handleKey(msg tea.KeyMsg) (threadListModel, tea.Cmd) {
	switch {
	case keyMatches(msg, km.Up):
		m.move(-1)
	case keyMatches(msg, km.Down):
		m.move(1)
	case keyMatches(msg, km.Top):
		m.cursor = 0
	case keyMatches(msg, km.Bottom):
		m.cursor = len(m.st.Threads) - 1
	case keyMatches(msg, km.Enter):
		if m.state == listReady && len(m.st.Threads) > 0 {
			th := m.st.Threads[m.cursor]
			return m, navCmd(ScreenReader, th)
		}
	case keyMatches(msg, km.Back):
		return m, navCmd(ScreenForum, nil)
	case keyMatches(msg, km.NextPg):
		return m.gotoPage(m.st.ListPage + 1)
	case keyMatches(msg, km.PrevPg):
		return m.gotoPage(m.st.ListPage - 1)
	case keyMatches(msg, km.Refresh):
		if m.state != listLoading {
			m.state = listLoading
			m.err = nil
			return m, loadThreadsCmd(m.st)
		}
	case keyMatches(msg, km.Sort):
		if m.st.ListSearchKey == "" {
			if m.st.ListOrderBy == "lastpostdesc" {
				m.st.ListOrderBy = "postdatedesc"
			} else {
				m.st.ListOrderBy = "lastpostdesc"
			}
			m.st.ListPage = 1
			m.state = listLoading
			m.err = nil
			return m, loadThreadsCmd(m.st)
		}
	case keyMatches(msg, km.Search):
		return m, navCmd(ScreenSearch, searchScopeThread)
	}
	m.syncViewport()
	return m, nil
}

// gotoPage 跳转页面（超出范围时忽略）。
func (m threadListModel) gotoPage(page int) (threadListModel, tea.Cmd) {
	if page < 1 || (m.st.ListPages > 0 && page > m.st.ListPages) {
		return m, nil
	}
	m.st.ListPage = page
	m.state = listLoading
	m.err = nil
	return m, loadThreadsCmd(m.st)
}

func (m *threadListModel) move(delta int) {
	n := len(m.st.Threads)
	if n == 0 {
		return
	}
	m.cursor = (m.cursor + delta + n) % n
}

func (m *threadListModel) syncViewport() {
	if m.state != listReady {
		return
	}
	m.vp.SetContent(m.renderList())
	if len(m.st.Threads) == 0 {
		return
	}
	visible := m.vp.Height
	if visible <= 0 {
		return
	}
	// 列表头占 2 行，每帖占 2 行（标题 + meta）
	cur := 2 + m.cursor*2
	if cur < m.vp.YOffset {
		m.vp.SetYOffset(cur)
	} else if cur >= m.vp.YOffset+visible {
		m.vp.SetYOffset(cur - visible + 1)
	}
}

func (m threadListModel) View() string {
	switch m.state {
	case listLoading:
		return fmt.Sprintf("\n  %s 正在加载帖子…", m.sp.View())
	case listError:
		hint := "按 r 重试"
		if isLoginError(m.err) {
			hint = "登录已失效或未登录，按 L 重新登录"
		}
		return fmt.Sprintf(
			"\n  %s\n\n  %s\n\n  %s\n",
			errorStyle.Render("加载帖子失败"),
			dimStyle.Render(m.err.Error()),
			dimStyle.Render(hint),
		)
	}
	if len(m.st.Threads) == 0 {
		return "\n  " + dimStyle.Render("没有帖子")
	}
	return m.vp.View()
}

// renderList 渲染帖子列表（首行为版面标题头）。
func (m threadListModel) renderList() string {
	var sb strings.Builder
	if m.st.CurrentForum != nil {
		title := m.st.CurrentForum.Name
		if m.st.ListSearchKey != "" {
			title += " · 搜索 \"" + m.st.ListSearchKey + "\""
		}
		title += fmt.Sprintf("  %d/%d 页", m.st.ListPage, m.st.ListPages)
		sb.WriteString(truncateLine(headerStyle.Render(" "+title), m.width))
		sb.WriteString("\n\n")
	}
	for i, th := range m.st.Threads {
		var line string
		if i == m.cursor {
			line = "  " + selectedStyle.Render("▸ "+th.Subject)
		} else {
			line = "    " + th.Subject
		}
		meta := fmt.Sprintf("%s · %d回复 · %s", th.Author, th.Replies, formatTime(th.LastPost))
		meta = dimStyle.Render(meta)
		// 标题与 meta 分行显示，更易读
		sb.WriteString(truncateLine(line, m.width))
		sb.WriteString("\n")
		sb.WriteString(truncateLine("      "+meta, m.width))
		sb.WriteString("\n")
	}
	return sb.String()
}
