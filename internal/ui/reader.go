package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/bbcode"
	"github.com/ArkURL/nga-tui/internal/debug"
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
	// floorLines 记录每层楼头部在渲染内容中的起始行号（供 j/k 按楼跳转）。
	floorLines []int
}

type contentLoadedMsg struct {
	tid int // 请求时对应的帖子 tid，用于丢弃过期响应
	res *api.ThreadContentResult
	err error
}

func newReaderModel() readerModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	return readerModel{
		state: readerLoading,
		sp:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		vp:    vp,
	}
}

func loadContentCmd(st *State) tea.Cmd {
	tid := st.CurrentThread.TID // 请求发起时的目标帖
	return func() tea.Msg {
		res, err := st.Client.GetThreadContent(tid, st.ReadPage)
		return contentLoadedMsg{tid: tid, res: res, err: err}
	}
}

func (m readerModel) Init() tea.Cmd {
	if m.st == nil || m.st.CurrentThread == nil {
		return nil
	}
	m.state = readerLoading
	return tea.Batch(m.sp.Tick, loadContentCmd(m.st))
}

// start 重置阅读视图到加载态并加载当前帖（指针方法，App 导航时调用，
// 修复 Init() 值接收者导致的旧内容残留闪屏）。
func (m *readerModel) start() tea.Cmd {
	if m.st == nil || m.st.CurrentThread == nil {
		return nil
	}
	m.state = readerLoading
	m.err = nil
	m.vp.GotoTop() // 切换帖子时重置滚动位置
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
		// 丢弃过期响应（用户已切换到其它帖子）
		if m.st == nil || m.st.CurrentThread == nil || msg.tid != m.st.CurrentThread.TID {
			return m, nil
		}
		if msg.err != nil {
			m.state = readerError
			m.err = msg.err
			if isLoginError(msg.err) && m.st != nil {
				m.st.LoggedIn = false
				debug.Logf("帖子内容加载被拒（登录失效）: %v", msg.err)
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

	case tea.MouseMsg:
		// 支持鼠标滚轮滚动楼层
		if m.state == readerReady {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m readerModel) handleKey(msg tea.KeyMsg) (readerModel, tea.Cmd) {
	switch {
	case keyMatches(msg, km.Back):
		return m, navCmd(ScreenThreadList, nil)
	case keyMatches(msg, km.Up):
		// k/↑：跳到上一楼头部
		m.vp.SetYOffset(m.prevFloorLine())
	case keyMatches(msg, km.Down):
		// j/↓：跳到下一楼头部
		m.vp.SetYOffset(m.nextFloorLine())
	case keyMatches(msg, km.ScrollUp):
		// Shift+K：向上细调一行
		m.vp.LineUp(1)
	case keyMatches(msg, km.ScrollDown):
		// Shift+J：向下细调一行
		m.vp.LineDown(1)
	case keyMatches(msg, km.Top):
		m.vp.GotoTop()
	case keyMatches(msg, km.Bottom):
		m.vp.GotoBottom()
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

// renderReplies 渲染当前页所有楼层，并记录每楼头部在内容中的起始行号。
func (m *readerModel) renderReplies() string {
	var sb strings.Builder
	line := 0
	m.floorLines = m.floorLines[:0]
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
		header = truncateLine(header, m.width)
		m.floorLines = append(m.floorLines, line)
		sb.WriteString(header)
		sb.WriteString("\n")
		line += 1 + strings.Count(header, "\n")

		// 内容（BBCode 渲染并按宽度折行）
		body := bbcode.Render(r.Content, max(m.width-2, 20))
		sb.WriteString(body)
		sb.WriteString("\n\n")
		line += strings.Count(body, "\n") + 2

		if i < len(m.st.Replies)-1 {
			sb.WriteString(dimStyle.Render(strings.Repeat("─", max(m.width-2, 10))))
			sb.WriteString("\n\n")
			line += 2
		}
	}
	return sb.String()
}

// nextFloorLine 返回当前视口位置之后的下一楼头部行号；没有则返回当前偏移。
func (m *readerModel) nextFloorLine() int {
	top := m.vp.YOffset
	for _, l := range m.floorLines {
		if l > top {
			return l
		}
	}
	return top
}

// prevFloorLine 返回当前视口位置之前的上一楼头部行号。
func (m *readerModel) prevFloorLine() int {
	top := m.vp.YOffset
	var prev int
	for _, l := range m.floorLines {
		if l < top {
			prev = l
		} else {
			break
		}
	}
	return prev
}
