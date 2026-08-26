package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/model"
)

type searchModel struct {
	st    *State
	scope searchScope
	width int
	input textinput.Model

	// 版面搜索：输入后显示本地匹配结果
	showResults bool
	results     []model.Forum
	cursor      int
	err         error
}

func newSearchModel() searchModel {
	m := searchModel{}
	m.input = textinput.New()
	m.input.Prompt = "搜索> "
	m.input.CharLimit = 50
	m.input.Width = 60
	return m
}

func (m searchModel) Init() tea.Cmd { return nil }

// reset 重置搜索输入并聚焦（指针方法，App 导航时调用）。
func (m *searchModel) reset() tea.Cmd {
	if m.scope == searchScopeForum {
		m.input.Placeholder = "输入版面名称（如 魔兽）"
	} else {
		m.input.Placeholder = "输入关键字，在当前版面内搜帖"
	}
	m.showResults = false
	m.err = nil
	return m.input.Focus()
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		if m.showResults {
			// 结果列表无文本输入，q/h/esc 均可返回
			if keyMatches(msg, km.Back) {
				return m.back()
			}
			return m.updateResults(msg)
		}
		// 输入态：只用 Esc 返回，避免 q/h 无法输入
		if isEsc(msg) {
			return m.back()
		}
		if keyMatches(msg, km.Enter) {
			return m.submit()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// back 返回上一级（输入态返回来源视图；结果态回到输入）。
func (m searchModel) back() (searchModel, tea.Cmd) {
	if m.showResults {
		m.showResults = false
		return m, nil
	}
	if m.scope == searchScopeThread {
		return m, navCmd(ScreenThreadList, nil)
	}
	return m, navCmd(ScreenForum, nil)
}

// submit 提交关键字。
func (m searchModel) submit() (searchModel, tea.Cmd) {
	kw := strings.TrimSpace(m.input.Value())
	if kw == "" {
		return m, nil
	}
	if m.scope == searchScopeThread {
		// 版内搜帖：设置搜索状态并标记需要重新加载
		m.st.ListSearchKey = kw
		m.st.ListPage = 1
		m.st.ListReload = true
		return m, navCmd(ScreenThreadList, nil)
	}
	// 版面搜索：本地过滤已加载的分类
	m.results = filterForums(m.st.Categories, kw)
	m.cursor = 0
	m.showResults = true
	if len(m.results) == 0 {
		m.err = fmt.Errorf("没有匹配「%s」的版面", kw)
	} else {
		m.err = nil
	}
	return m, nil
}

func (m searchModel) updateResults(msg tea.KeyMsg) (searchModel, tea.Cmd) {
	switch {
	case keyMatches(msg, km.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case keyMatches(msg, km.Down):
		if m.cursor < len(m.results)-1 {
			m.cursor++
		}
	case keyMatches(msg, km.Enter):
		if len(m.results) > 0 {
			f := m.results[m.cursor]
			return m, navCmd(ScreenThreadList, f)
		}
	case keyMatches(msg, km.Favorite):
		if len(m.results) > 0 && m.st != nil {
			f := m.results[m.cursor]
			k := f.BoardKey()
			if _, ok := m.st.Favorites[k]; ok {
				delete(m.st.Favorites, k)
			} else {
				m.st.Favorites[k] = model.BoardRef{FID: f.FID, STID: f.STID, Name: f.Name}
			}
			persistAll(m.st.Client.Cookies(), m.st.Favorites)
		}
	case keyMatches(msg, km.Search):
		m.showResults = false
	}
	return m, nil
}

// filterForums 在已加载的分类数据中按名称/简介匹配版面。
func filterForums(cats []model.Category, kw string) []model.Forum {
	kw = strings.ToLower(kw)
	var out []model.Forum
	for _, cat := range cats {
		for _, g := range cat.Groups {
			for _, f := range g.Forums {
				if strings.Contains(strings.ToLower(f.Name), kw) ||
					strings.Contains(strings.ToLower(f.Info), kw) {
					out = append(out, f)
				}
			}
		}
	}
	return out
}

func (m searchModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	if m.scope == searchScopeForum {
		sb.WriteString(titleStyle.Render(" 搜索版面"))
	} else {
		cur := "?"
		if m.st != nil && m.st.CurrentForum != nil {
			cur = m.st.CurrentForum.Name
		}
		sb.WriteString(fmt.Sprintf(" %s（当前版面：%s）", titleStyle.Render("搜索帖子"), cur))
	}
	sb.WriteString("\n\n")
	sb.WriteString("  " + m.input.View() + "\n\n")

	if m.err != nil {
		sb.WriteString("  " + dimStyle.Render(m.err.Error()) + "\n")
	}
	if m.showResults {
		if len(m.results) == 0 {
			sb.WriteString("  " + dimStyle.Render("无匹配结果，修改关键字后回车重试") + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n\n", dimStyle.Render(fmt.Sprintf("匹配 %d 个版面，回车进入 · f 收藏", len(m.results)))))
			for i, f := range m.results {
				mark := "  "
				if m.st != nil {
					if _, ok := m.st.Favorites[f.BoardKey()]; ok {
						mark = "★ "
					}
				}
				line := "    " + mark + f.Name
				if f.Info != "" {
					line += " " + dimStyle.Render("· "+f.Info)
				}
				if i == m.cursor {
					line = "  " + selectedStyle.Render("▸ "+mark+f.Name)
					if f.Info != "" {
						line += " " + dimStyle.Render("· "+f.Info)
					}
				}
				sb.WriteString(truncateLine(line, m.width))
				sb.WriteString("\n")
			}
		}
	}
	if m.showResults {
		sb.WriteString("\n  " + dimStyle.Render("Enter 进入 · f 收藏/取消 · Esc 返回"))
	} else {
		sb.WriteString("\n  " + dimStyle.Render("回车搜索 · Esc 返回"))
	}
	return sb.String()
}
