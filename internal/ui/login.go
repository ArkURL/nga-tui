package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/browser"
)

// loginMode 表示登录视图的当前模式。
type loginMode int

const (
	loginChoice loginMode = iota // 选择登录方式
	loginBrowser                 // 浏览器自动抓取 cookie 进行中
	loginManual                  // 手动粘贴 cookie
)

type loginModel struct {
	client    *api.Client
	mode      loginMode
	manual    textinput.Model
	capturing bool // 浏览器自动抓取 cookie 进行中
	sp        spinner.Model
	err       error
	// confirmLogout 等待确认登出（防止误按 X 清掉会话）。
	confirmLogout bool
	// onSuccess 登录成功后的回调（App 注入：设置 cookie + 保存配置）。
	onSuccess func(*api.Session)
	// onLogout 登出回调（App 注入）。
	onLogout func()
}

type loginResultMsg struct {
	sess *api.Session
	err  error
}

func newLoginModel() loginModel {
	m := loginModel{sp: spinner.New(spinner.WithSpinner(spinner.Dot))}
	m.manual = textinput.New()
	m.manual.Prompt = "Cookie> "
	m.manual.Placeholder = "粘贴 ngaPassportUid=xxx; ngaPassportCid=xxx"
	m.manual.CharLimit = 500
	m.manual.Width = 80
	return m
}

func (m loginModel) Init() tea.Cmd { return nil }

// reset 重置登录视图到"选择方式"（指针方法，App 导航时调用）。
func (m *loginModel) reset() tea.Cmd {
	m.mode = loginChoice
	m.capturing = false
	m.confirmLogout = false
	m.err = nil
	m.manual.Blur()
	return nil
}

func (m loginModel) Update(msg tea.Msg) (loginModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isEsc(msg) {
			return m, navCmd(ScreenForum, nil)
		}
		if m.capturing {
			return m, nil // 抓取中忽略按键
		}
		switch m.mode {
		case loginChoice:
			return m.updateChoice(msg)
		case loginManual:
			return m.updateManual(msg)
		}

	case loginResultMsg:
		m.capturing = false
		m.mode = loginChoice
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if m.onSuccess != nil {
			m.onSuccess(msg.sess)
		}
		return m, navCmd(ScreenForum, nil)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateChoice 处理登录方式选择。
func (m loginModel) updateChoice(msg tea.KeyMsg) (loginModel, tea.Cmd) {
	// 确认登出状态：Y 确认，其他键取消
	if m.confirmLogout {
		m.confirmLogout = false
		if msg.String() == "y" || msg.String() == "Y" {
			if m.client != nil && m.client.LoggedIn() && m.onLogout != nil {
				m.onLogout()
				m.err = nil
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "B":
		m.mode = loginBrowser
		m.capturing = true
		m.err = nil
		return m, browserCaptureCmd(2 * time.Minute)
	case "M":
		m.mode = loginManual
		m.err = nil
		return m, m.manual.Focus()
	case "X", "x":
		// 先进入确认状态，防止误触登出
		m.confirmLogout = true
		m.err = nil
		return m, nil
	}
	return m, nil
}

func (m loginModel) updateManual(msg tea.KeyMsg) (loginModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		cookies, err := parseCookies(m.manual.Value())
		if err != nil {
			m.err = err
			return m, nil
		}
		sess := &api.Session{UID: cookies["ngaPassportUid"], Cookies: cookies}
		if m.onSuccess != nil {
			m.onSuccess(sess)
		}
		return m, navCmd(ScreenForum, nil)
	case "tab", "up", "esc":
		m.manual.Blur()
		m.mode = loginChoice
		m.err = nil
		return m, nil
	}
	var cmd tea.Cmd
	m.manual, cmd = m.manual.Update(msg)
	return m, cmd
}

// browserCaptureCmd 启动浏览器自动登录。抓取到 cookie 后先用登录门槛接口
// 验证会话确实可用，验证通过才返回（避免保存到半登录/失效的 cookie）。
func browserCaptureCmd(timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		validate := func(cookies map[string]string) bool {
			c := api.NewClient()
			c.SetCookies(cookies)
			ok, err := c.CheckLogin()
			return err == nil && ok
		}
		cookies, err := browser.CaptureNGACookies(timeout, validate)
		if err != nil {
			return loginResultMsg{err: err}
		}
		sess := &api.Session{
			UID:     cookies[browser.UidCookie],
			Cookies: cookies,
		}
		return loginResultMsg{sess: sess}
	}
}

// parseCookies 解析 "k=v; k2=v2" 形式的 cookie 串。
func parseCookies(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	if out["ngaPassportUid"] == "" || out["ngaPassportCid"] == "" {
		return nil, fmt.Errorf("cookie 中缺少 ngaPassportUid 或 ngaPassportCid")
	}
	return out, nil
}

func (m loginModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n  " + titleStyle.Render("登录 NGA") + "\n\n")

	switch m.mode {
	case loginChoice:
		if m.confirmLogout {
			sb.WriteString("  " + errorStyle.Render("确认登出？") + "\n\n")
			sb.WriteString("  " + dimStyle.Render("登出将清除本地保存的登录 Cookie") + "\n\n")
			sb.WriteString("  " + dimStyle.Render("按 Y 确认登出 · 其他键取消") + "\n")
			break
		}
		if m.client != nil && m.client.LoggedIn() {
			sb.WriteString("  " + okStyle.Render("当前已登录") + "\n\n")
			sb.WriteString("  " + dimStyle.Render("如需切换账号，请选择重新登录：") + "\n\n")
			sb.WriteString("  " + titleStyle.Render("B") + "  重新登录（推荐）\n")
			sb.WriteString("      " + dimStyle.Render("打开独立 Chrome 窗口，登录后自动抓取并保存 Cookie") + "\n\n")
			sb.WriteString("  " + titleStyle.Render("X") + "  登出\n\n")
		} else {
			sb.WriteString("  " + dimStyle.Render("NGA 网页登录需要验证码，无法在终端内完成，请选择：") + "\n\n")
			sb.WriteString("  " + titleStyle.Render("B") + "  浏览器登录（推荐）\n")
			sb.WriteString("      " + dimStyle.Render("打开独立 Chrome 窗口，登录后自动抓取并保存 Cookie") + "\n\n")
			sb.WriteString("  " + titleStyle.Render("M") + "  手动粘贴 Cookie\n")
			sb.WriteString("      " + dimStyle.Render("从浏览器开发者工具复制 ngaPassportUid/ngaPassportCid 粘贴") + "\n\n")
		}
		if m.err != nil {
			sb.WriteString("  " + errorStyle.Render(m.err.Error()) + "\n\n")
		}
		sb.WriteString("  " + dimStyle.Render("B / M / X 选择 · Esc 返回") + "\n")

	case loginBrowser:
		sb.WriteString("  " + m.sp.View() + " 正在打开浏览器…\n\n")
		sb.WriteString("  " + dimStyle.Render("已打开一个独立的 Chrome 窗口，请在窗口中登录 bbs.nga.cn。") + "\n")
		sb.WriteString("  " + dimStyle.Render("登录完成后将自动抓取 Cookie 并保存，最多等待 2 分钟。") + "\n\n")
		if m.err != nil {
			sb.WriteString("  " + errorStyle.Render(m.err.Error()) + "\n\n")
		}

	case loginManual:
		sb.WriteString("  " + dimStyle.Render("在浏览器登录 bbs.nga.cn 后，从开发者工具复制 cookie 粘贴到下面：") + "\n\n")
		sb.WriteString("  " + m.manual.View() + "\n\n")
		if m.err != nil {
			sb.WriteString("  " + errorStyle.Render(m.err.Error()) + "\n\n")
		}
		sb.WriteString("  " + dimStyle.Render("回车登录 · Esc 返回") + "\n")
	}
	return sb.String()
}
