package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/model"
)

type forumItemKind int

const (
	itemCategory forumItemKind = iota
	itemGroup
	itemForum
)

type forumItem struct {
	kind  forumItemKind
	cat   string
	group string
	forum model.Forum
}

type forumState int

const (
	forumLoading forumState = iota
	forumReady
	forumError
)

type forumModel struct {
	st      *State
	client  *api.Client
	state   forumState
	favOnly bool // 仅显示收藏版面
	items   []forumItem
	selIdx  []int // 可选中项的索引（指向 items）
	cursor  int   // 在 selIdx 中的位置
	width   int
	height  int
	sp      spinner.Model
	err     error
	vp      viewport.Model
}

type categoriesLoadedMsg struct {
	cats []model.Category
	err  error
}

func newForumModel() forumModel {
	return forumModel{
		state: forumLoading,
		sp:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		vp:    viewport.New(0, 0),
	}
}

func loadCategoriesCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		done := make(chan *categoriesLoadedMsg, 1)
		go func() {
			cats, err := c.GetCategories()
			done <- &categoriesLoadedMsg{cats: cats, err: err}
		}()
		select {
		case r := <-done:
			return *r
		case <-time.After(20 * time.Second):
			return categoriesLoadedMsg{err: fmt.Errorf("请求超时，请检查网络后按 r 重试")}
		}
	}
}

func (m forumModel) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, loadCategoriesCmd(m.client))
}

// start 重置版面视图到加载态并重新拉取分类（指针方法，App 导航时调用）。
func (m *forumModel) start() tea.Cmd {
	if m.client == nil {
		return nil
	}
	m.state = forumLoading
	m.err = nil
	return tea.Batch(m.sp.Tick, loadCategoriesCmd(m.client))
}

func (m forumModel) Update(msg tea.Msg) (forumModel, tea.Cmd) {
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

	case categoriesLoadedMsg:
		if msg.err != nil {
			m.state = forumError
			m.err = msg.err
			return m, nil
		}
		m.state = forumReady
		if m.st != nil {
			m.st.Categories = msg.cats
		}
		m.rebuild()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// rebuild 根据当前筛选（全部/收藏）重建版面列表。
func (m *forumModel) rebuild() {
	if m.st == nil {
		return
	}
	if m.favOnly {
		m.items = buildFavItems(m.st.Favorites)
		m.backfillFavNames()
	} else {
		m.items = buildForumItems(m.st.Categories)
	}
	m.selIdx = selectableIndices(m.items)
	if m.cursor >= len(m.selIdx) {
		m.cursor = max(len(m.selIdx)-1, 0)
	}
	m.syncViewport()
}

func (m forumModel) handleKey(msg tea.KeyMsg) (forumModel, tea.Cmd) {
	switch {
	case keyMatches(msg, km.Up):
		m.move(-1)
	case keyMatches(msg, km.Down):
		m.move(1)
	case keyMatches(msg, km.Top):
		m.cursor = 0
	case keyMatches(msg, km.Bottom):
		m.cursor = len(m.selIdx) - 1
	case keyMatches(msg, km.Enter):
		if m.state == forumReady && len(m.selIdx) > 0 {
			f := m.items[m.selIdx[m.cursor]].forum
			return m, navCmd(ScreenThreadList, f)
		}
	case keyMatches(msg, km.Refresh):
		if m.state != forumLoading {
			m.state = forumLoading
			m.err = nil
			m.vp.SetContent("")
			return m, loadCategoriesCmd(m.client)
		}
	case keyMatches(msg, km.Search):
		return m, navCmd(ScreenSearch, searchScopeForum)
	case keyMatches(msg, km.Goto):
		return m, navCmd(ScreenSearch, searchScopeGoto)
	case keyMatches(msg, km.Favorite):
		if m.state == forumReady && len(m.selIdx) > 0 && m.st != nil {
			f := m.items[m.selIdx[m.cursor]].forum
			k := f.BoardKey()
			added := false
			if _, ok := m.st.Favorites[k]; ok {
				delete(m.st.Favorites, k)
			} else {
				m.st.Favorites[k] = model.BoardRef{FID: f.FID, STID: f.STID, Name: f.Name}
				added = true
			}
			persistAll(m.st.Client.Cookies(), m.st.Favorites)
			m.rebuild()
			name := f.Name
			if name == "" {
				name = k
			}
			if added {
				return m, statusCmd("已收藏「" + name + "」")
			}
			return m, statusCmd("已取消收藏「" + name + "」")
		}
	case keyMatches(msg, km.ToggleForum):
		if m.state == forumReady {
			m.favOnly = !m.favOnly
			m.cursor = 0
			m.rebuild()
			return m, nil
		}
	}
	m.syncViewport()
	return m, nil
}

// move 移动光标（可选中项之间循环）。
func (m *forumModel) move(delta int) {
	if len(m.selIdx) == 0 {
		return
	}
	n := len(m.selIdx)
	m.cursor = (m.cursor + delta + n) % n
}

// syncViewport 重设内容并保证光标行可见。
func (m *forumModel) syncViewport() {
	if m.state != forumReady {
		return
	}
	m.vp.SetContent(m.renderList())
	if len(m.selIdx) == 0 {
		return
	}
	visible := m.vp.Height
	if visible <= 0 {
		return
	}
	// 列表头占 2 行
	cur := m.selIdx[m.cursor] + 2
	if cur < m.vp.YOffset {
		m.vp.SetYOffset(cur)
	} else if cur >= m.vp.YOffset+visible {
		m.vp.SetYOffset(cur - visible + 1)
	}
}

func (m forumModel) View() string {
	switch m.state {
	case forumLoading:
		return fmt.Sprintf("\n  %s 正在加载版面分类…", m.sp.View())
	case forumError:
		return fmt.Sprintf(
			"\n  %s\n\n  %s\n\n  %s\n",
			errorStyle.Render("加载版面分类失败"),
			dimStyle.Render(m.err.Error()),
			dimStyle.Render("按 r 重试"),
		)
	}
	return m.vp.View()
}

// renderList 把 items 渲染为多行文本（首行为筛选模式标题头）。
func (m forumModel) renderList() string {
	var sb strings.Builder
	mode := "全部版面"
	if m.favOnly {
		mode = "收藏版面"
	}
	sb.WriteString(truncateLine(headerStyle.Render(" "+mode), m.width))
	sb.WriteString("\n\n")

	for i, it := range m.items {
		var line string
		switch it.kind {
		case itemCategory:
			line = " " + categoryStyle.Render("== "+it.cat)
		case itemGroup:
			line = "    " + groupHeaderStyle.Render(it.group)
		case itemForum:
			name := it.forum.Name
			if it.forum.STID != "" {
				name += " [合集]"
			}
			mark := "  "
			if m.st != nil {
				if _, ok := m.st.Favorites[it.forum.BoardKey()]; ok {
					mark = "★ "
				}
			}
			name = mark + name
			info := ""
			if it.forum.Info != "" {
				info = " " + dimStyle.Render("· "+it.forum.Info)
			}
			if m.state == forumReady && i == m.selIdx[m.cursor] {
				line = "  " + selectedStyle.Render("▸ "+name) + info
			} else {
				line = "    " + name + info
			}
		}
		sb.WriteString(truncateLine(line, m.width))
		sb.WriteString("\n")
	}
	if len(m.items) == 0 {
		sb.WriteString("  " + dimStyle.Render("暂无收藏版面，按 Tab 切回全部后按 f 收藏") + "\n")
	}
	return sb.String()
}

// buildForumItems 把分类数据扁平化为可滚动列表。
func buildForumItems(cats []model.Category) []forumItem {
	var items []forumItem
	for _, cat := range cats {
		items = append(items, forumItem{kind: itemCategory, cat: cat.Name})
		for _, g := range cat.Groups {
			if len(g.Forums) == 0 {
				continue
			}
			items = append(items, forumItem{kind: itemGroup, group: g.Name})
			for _, f := range g.Forums {
				items = append(items, forumItem{kind: itemForum, forum: f})
			}
		}
	}
	return items
}

// buildFavItems 直接渲染收藏列表（不再扫描分类树，因此子版面收藏也能显示）。
func buildFavItems(favs map[string]model.BoardRef) []forumItem {
	var items []forumItem
	for _, ref := range favs {
		items = append(items, forumItem{
			kind: itemForum,
			forum: model.Forum{
				FID:  ref.FID,
				STID: ref.STID,
				Name: ref.Name,
			},
		})
	}
	return items
}

// backfillFavNames 用分类树回填旧配置收藏（只有 fid、无 Name）的名称，查不到退化显示 Key。
func (m *forumModel) backfillFavNames() {
	if m.st == nil {
		return
	}
	nameByKey := map[string]string{}
	for _, cat := range m.st.Categories {
		for _, g := range cat.Groups {
			for _, f := range g.Forums {
				nameByKey[f.BoardKey()] = f.Name
			}
		}
	}
	for i := range m.items {
		f := &m.items[i].forum
		if f.Name == "" {
			if n, ok := nameByKey[f.BoardKey()]; ok {
				f.Name = n
			} else {
				f.Name = f.BoardKey()
			}
		}
	}
	// 名称回填后重排序
	sort.Slice(m.items, func(i, j int) bool {
		return m.items[i].forum.Name < m.items[j].forum.Name
	})
}

func selectableIndices(items []forumItem) []int {
	var idx []int
	for i, it := range items {
		if it.kind == itemForum {
			idx = append(idx, i)
		}
	}
	return idx
}
