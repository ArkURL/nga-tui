package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/bbcode"
)

type readerState int

const (
	readerLoading readerState = iota
	readerReady
	readerError
)

type readerModel struct {
	st     *State
	state  readerState
	width  int
	height int
	sp     spinner.Model
	vp     viewport.Model
	err    error
}

type contentLoadedMsg struct {
	res *api.ThreadContentResult
	err error
}

func newReaderModel() readerModel {
	return readerModel{
		state: readerLoading,
		sp:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		vp:    viewport.New(0, 0),
	}
}

func loadContentCmd(st *State) tea.Cmd {
	return func() tea.Msg {
		res, err := st.Client.GetThreadContent(st.CurrentThread.TID, st.ReadPage)
		return contentLoadedMsg{res: res, err: err}
	}
}

func (m readerModel) Init() tea.Cmd {
	if m.st == nil || m.st.CurrentThread == nil {
		return nil
	}
	m.state = readerLoading
	return tea.Batch(m.sp.Tick, loadContentCmd(m.st))
}

func (m readerModel) Update(msg tea.Msg) (readerModel, tea.Cmd) {
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
		if m.state == readerReady {
			m.syncViewport()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case contentLoadedMsg:
		if msg.err != nil {
			m.state = readerError
			m.err = msg.err
			if isLoginError(msg.err) && m.st != nil {
				m.st.LoggedIn = false
			}
			return m, nil
		}
		m.state = readerReady
		m.st.Replies = msg.res.Replies
		m.st.ReplyUsers = msg.res.Users
		m.st.ReadPage = msg.res.Page
		m.st.ReadPages = msg.res.Pages
		m.syncViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m readerModel) handleKey(msg tea.KeyMsg) (readerModel, tea.Cmd) {
	switch {
	case keyMatches(msg, km.Back):
		return m, navCmd(ScreenThreadList, nil)
	case keyMatches(msg, km.NextPg):
		return m.gotoPage(m.st.ReadPage + 1)
	case keyMatches(msg, km.PrevPg):
		return m.gotoPage(m.st.ReadPage - 1)
	case keyMatches(msg, km.Refresh):
		if m.state != readerLoading {
			m.state = readerLoading
			m.err = nil
			return m, loadContentCmd(m.st)
		}
	}
	m.syncViewport()
	return m, nil
}

func (m readerModel) gotoPage(page int) (readerModel, tea.Cmd) {
	if page < 1 || (m.st.ReadPages > 0 && page > m.st.ReadPages) {
		return m, nil
	}
	m.st.ReadPage = page
	m.state = readerLoading
	m.err = nil
	m.vp.GotoTop()
	return m, loadContentCmd(m.st)
}

func (m *readerModel) syncViewport() {
	if m.state != readerReady {
		return
	}
	m.vp.SetContent(m.renderReplies())
}

func (m readerModel) View() string {
	switch m.state {
	case readerLoading:
		return fmt.Sprintf("\n  %s 正在加载楼层…", m.sp.View())
	case readerError:
		hint := "按 r 重试，q 返回"
		if isLoginError(m.err) {
			hint = "登录已失效或未登录，按 L 重新登录"
		}
		return fmt.Sprintf(
			"\n  %s\n\n  %s\n\n  %s\n",
			errorStyle.Render("加载帖子内容失败"),
			dimStyle.Render(m.err.Error()),
			dimStyle.Render(hint),
		)
	}
	return m.vp.View()
}

// renderReplies 渲染当前页所有楼层。
func (m readerModel) renderReplies() string {
	var sb strings.Builder
	for i, r := range m.st.Replies {
		u := m.st.ReplyUsers[r.AuthorID]
		author := u.Username
		if author == "" {
			author = fmt.Sprintf("UID:%d", r.AuthorID)
		}

		// 楼层头
		header := fmt.Sprintf("#%d  %s  %s", r.Lou, author, r.PostDate)
		if r.Lou == 0 {
			header = titleStyle.Render(m.st.CurrentThread.Subject) + "\n" + header
		}
		sb.WriteString(truncateLine(header, m.width))
		sb.WriteString("\n")

		// 内容（BBCode 渲染并按宽度折行）
		body := bbcode.Render(r.Content, max(m.width-2, 20))
		sb.WriteString(body)
		sb.WriteString("\n\n")

		if i < len(m.st.Replies)-1 {
			sb.WriteString(dimStyle.Render(strings.Repeat("─", max(m.width-2, 10))))
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}
