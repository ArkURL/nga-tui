package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/debug"
	"github.com/ArkURL/nga-tui/internal/model"
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
	// showSubs 为 true 时显示子版面列表（默认帖子视图，按 t 切换）。
	showSubs bool
	// cursorLines 记录每个可选中项首行在渲染内容中的行号。
	cursorLines []int
}

type threadsLoadedMsg struct {
	fid  string // 请求时对应的版面 fid，用于丢弃过期响应
	stid string // 请求时对应的合集 stid
	key  string // 请求时对应的搜索关键字
	res  *api.ThreadListResult
	err  error
}

func newThreadListModel() threadListModel {
	return threadListModel{
		state: listLoading,
		sp:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		vp:    viewport.New(0, 0),
	}
}

func loadThreadsCmd(st *State) tea.Cmd {
	fid := st.CurrentForum.FID // 请求发起时的目标版面（取 CurrentForum 值，非发送参数）
	stid := st.CurrentForum.STID
	key := st.ListSearchKey
	page := st.ListPage
	order := st.ListOrderBy
	return func() tea.Msg {
		res, err := st.Client.GetThreads(fid, stid, page, order, key)
		return threadsLoadedMsg{fid: fid, stid: stid, key: key, res: res, err: err}
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

// start 重置帖子列表到加载态并重新加载当前版面（指针方法，App 导航时调用，
// 修复 Init() 值接收者导致的旧列表残留闪屏）。
func (m *threadListModel) start() tea.Cmd {
	if m.st == nil || m.st.CurrentForum == nil {
		return nil
	}
	m.state = listLoading
	m.err = nil
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
		// 丢弃过期响应（用户已切换版面/合集/搜索条件）
		if m.st == nil || m.st.CurrentForum == nil ||
			msg.fid != m.st.CurrentForum.FID ||
			msg.stid != m.st.CurrentForum.STID ||
			msg.key != m.st.ListSearchKey {
			return m, nil
		}
		if msg.err != nil {
			m.state = listError
			m.err = msg.err
			if isLoginError(msg.err) && m.st != nil {
				m.st.LoggedIn = false
				debug.Logf("帖子列表加载被拒（登录失效）: %v", msg.err)
			}
			return m, nil
		}
		m.state = listReady
		m.st.Threads = msg.res.Threads
		m.st.SubForums = filterSelfSubForums(msg.res.SubForums, m.st.CurrentForum)
		m.st.ListPage = msg.res.Page
		m.st.ListPages = msg.res.Pages
		// 直达/钻取时版面名可能只是 id，用响应里的真实名称回填（标题与收藏名）
		if msg.res.BoardName != "" && m.st.CurrentForum != nil {
			m.st.CurrentForum.Name = msg.res.BoardName
		}
		m.cursor = 0
		m.showSubs = false // 新加载默认回到帖子视图
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
		if n := len(m.cursorLines); n > 0 {
			m.cursor = n - 1
		}
	case keyMatches(msg, km.Enter):
		if m.state == listReady && len(m.cursorLines) > 0 {
			if m.showSubs {
				// 子版面视图：钻取子版面/合集
				sf := m.st.SubForums[m.cursor]
				if m.st.CurrentForum != nil {
					m.st.BoardStack = append(m.st.BoardStack, *m.st.CurrentForum)
				}
				target := model.Forum{FID: sf.ID, STID: sf.ID, Name: sf.Name}
				if !sf.IsCollection {
					target.FID = sf.ID
					target.STID = ""
				}
				return m, navCmd(ScreenThreadList, target)
			}
			th := m.st.Threads[m.cursor]
			return m, navCmd(ScreenReader, th)
		}
	case keyMatches(msg, km.ToggleSubs):
		// 子版面/帖子 视图切换（搜索时无子版面）
		if m.state == listReady && m.st != nil && m.st.ListSearchKey == "" && len(m.st.SubForums) > 0 {
			m.showSubs = !m.showSubs
			m.cursor = 0
		}
	case keyMatches(msg, km.Back):
		if m.showSubs {
			// 子版面视图先切回帖子视图
			m.showSubs = false
			m.cursor = 0
		} else if n := len(m.st.BoardStack); n > 0 {
			// 弹出父版面并重载
			parent := m.st.BoardStack[n-1]
			m.st.BoardStack = m.st.BoardStack[:n-1]
			m.st.CurrentForum = &parent
			m.st.ListSearchKey = ""
			m.st.ListPage = 1
			return m, m.start()
		} else {
			return m, navCmd(ScreenForum, nil)
		}
	case keyMatches(msg, km.Favorite):
		if m.state == listReady && m.st != nil {
			var ref model.BoardRef
			var name string
			if m.showSubs {
				ref = m.st.SubForums[m.cursor].BoardRef()
				name = m.st.SubForums[m.cursor].Name
			} else if cur := m.st.CurrentForum; cur != nil {
				ref = model.BoardRef{FID: cur.FID, STID: cur.STID, Name: cur.Name}
				name = cur.Name
			}
			if ref.Key() != "" {
				added := false
				if _, ok := m.st.Favorites[ref.Key()]; ok {
					delete(m.st.Favorites, ref.Key())
				} else {
					m.st.Favorites[ref.Key()] = ref
					added = true
				}
				persistAll(m.st.Client.Cookies(), m.st.Favorites)
				if name == "" {
					name = ref.Key()
				}
				if added {
					return m, statusCmd("已收藏「" + name + "」")
				}
				return m, statusCmd("已取消收藏「" + name + "」")
			}
		}
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

// hasSubForums 判断当前版面是否带子版面（搜索时不算）。
func (m *threadListModel) hasSubForums() bool {
	return m.st != nil && m.st.ListSearchKey == "" && len(m.st.SubForums) > 0
}

// filterSelfSubForums 过滤掉指向当前版面自身的子版面（合集打开时 __F.sub_forums
// 可能包含自己，避免无限自钻取）。
func filterSelfSubForums(subs []model.SubForum, cur *model.Forum) []model.SubForum {
	if cur == nil {
		return subs
	}
	curKey := cur.STID
	if curKey == "" {
		curKey = cur.FID
	}
	out := subs[:0]
	for _, sf := range subs {
		if sf.ID != curKey {
			out = append(out, sf)
		}
	}
	return out
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
	n := len(m.cursorLines)
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
	if len(m.cursorLines) == 0 {
		return
	}
	visible := m.vp.Height
	if visible <= 0 {
		return
	}
	cur := m.cursorLines[m.cursor]
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
	if m.showSubs {
		if len(m.st.SubForums) == 0 {
			return "\n  " + dimStyle.Render("该版面没有子版面")
		}
	} else if len(m.st.Threads) == 0 {
		return "\n  " + dimStyle.Render("没有帖子")
	}
	return m.vp.View()
}

// renderList 渲染当前视图：默认帖子列表，showSubs 时只渲染子版面列表。
func (m *threadListModel) renderList() string {
	var sb strings.Builder
	line := 0
	m.cursorLines = m.cursorLines[:0]
	if m.st.CurrentForum != nil {
		title := m.st.CurrentForum.Name
		if m.showSubs {
			title += " · 子版面"
		} else {
			if m.st.ListSearchKey != "" {
				title += " · 搜索 \"" + m.st.ListSearchKey + "\""
			}
			title += fmt.Sprintf("  %d/%d 页", m.st.ListPage, m.st.ListPages)
		}
		sb.WriteString(truncateLine(headerStyle.Render(" "+title), m.width))
		sb.WriteString("\n\n")
		line += 2
	}

	if m.showSubs {
		// 子版面视图：每项 1 行
		for _, sf := range m.st.SubForums {
			name := sf.Name
			if sf.IsCollection {
				name += " [合集]"
			}
			var l string
			if m.cursor == len(m.cursorLines) {
				l = "  " + selectedStyle.Render("▸ "+name)
			} else {
				l = "    " + name
			}
			if sf.Info != "" {
				l += " " + dimStyle.Render("· "+sf.Info)
			}
			m.cursorLines = append(m.cursorLines, line)
			sb.WriteString(truncateLine(l, m.width))
			sb.WriteString("\n")
			line++
		}
		return sb.String()
	}

	// 帖子视图：每帖 2 行（标题 + meta）
	for _, th := range m.st.Threads {
		var l string
		idx := len(m.cursorLines)
		if idx == m.cursor {
			l = "  " + selectedStyle.Render("▸ "+th.Subject)
		} else {
			l = "    " + th.Subject
		}
		meta := fmt.Sprintf("%s · %d回复 · %s", th.Author, th.Replies, formatTime(th.LastPost))
		meta = dimStyle.Render(meta)
		// 标题与 meta 分行显示，更易读
		m.cursorLines = append(m.cursorLines, line)
		sb.WriteString(truncateLine(l, m.width))
		sb.WriteString("\n")
		sb.WriteString(truncateLine("      "+meta, m.width))
		sb.WriteString("\n")
		line += 2
	}
	return sb.String()
}
