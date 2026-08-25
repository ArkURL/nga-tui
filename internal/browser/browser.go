// Package browser 通过 Chrome DevTools Protocol (CDP) 驱动浏览器，
// 在用户完成 NGA 网页登录后自动抓取 passport cookie。
package browser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// NGA passport cookie 名称。
const (
	UidCookie = "ngaPassportUid"
	CidCookie = "ngaPassportCid"
)

// NGALoginURL 是打开给用户登录的地址。
const NGALoginURL = "https://bbs.nga.cn/"

// CaptureNGACookies 启动一个独立的 Chrome 实例打开 NGA 登录页，
// 轮询直到用户在浏览器中登录成功。当捕获到 passport cookie 后，
// 会调用 validate 确认会话可用才返回（否则继续轮询等待）。
// timeout 为最长等待时间（含用户登录时间）。
func CaptureNGACookies(timeout time.Duration, validate func(map[string]string) bool) (map[string]string, error) {
	chrome, err := findChrome()
	if err != nil {
		return nil, err
	}
	profile, err := os.MkdirTemp("", "nga-tui-chrome-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(profile)

	cmd := exec.Command(chrome,
		"--remote-debugging-port=0",
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		NGALoginURL,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Chrome 失败: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	port, err := waitForDevToolsPort(profile, 20*time.Second)
	if err != nil {
		return nil, err
	}

	conn, err := connectPage(port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := &cdpClient{conn: conn}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cookies, err := client.getAllCookies()
		if err == nil {
			if got := extractCookies(cookies); got != nil {
				if validate == nil || validate(got) {
					return got, nil
				}
				// cookie 已捕获但会话验证未通过（可能登录未完全生效），继续轮询
			}
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("等待登录超时（%v）：请在浏览器中完成登录", timeout.Round(time.Second))
}

// findChrome 在常见路径中寻找 Chrome/Chromium 可执行文件。
func findChrome() (string, error) {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		paths = []string{
			"/usr/bin/google-chrome", "/usr/bin/google-chrome-stable",
			"/usr/bin/chromium", "/usr/bin/chromium-browser",
		}
	case "windows":
		paths = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 Chrome/Chromium（当前仅支持按 B 浏览器登录），请使用 M 手动粘贴 Cookie")
}

// waitForDevToolsPort 读取 Chrome 写入的 DevToolsActivePort 文件（随机调试端口）。
func waitForDevToolsPort(profile string, timeout time.Duration) (string, error) {
	portFile := filepath.Join(profile, "DevToolsActivePort")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(portFile)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 1 && strings.TrimSpace(lines[0]) != "" {
				return strings.TrimSpace(lines[0]), nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("Chrome 调试端口未就绪")
}

// connectPage 通过 /json 端点找到页面 target 并建立 WebSocket 连接。
func connectPage(port string) (*websocket.Conn, error) {
	httpURLs := []string{
		"http://127.0.0.1:" + port + "/json",
		"http://127.0.0.1:" + port + "/json/list",
	}
	var lastErr error
	for _, u := range httpURLs {
		resp, err := http.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		var targets []struct {
			Type                 string `json:"type"`
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&targets)
		resp.Body.Close()
		if decErr != nil {
			lastErr = decErr
			continue
		}
		for _, t := range targets {
			if t.Type == "page" && t.WebSocketDebuggerURL != "" {
				conn, _, err := websocket.DefaultDialer.Dial(t.WebSocketDebuggerURL, nil)
				if err != nil {
					lastErr = err
					continue
				}
				return conn, nil
			}
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("无法连接 Chrome 调试端点: %w", lastErr)
	}
	return nil, fmt.Errorf("Chrome 中没有可用的页面 target")
}

// cdpClient 是最小化的 CDP 客户端（仅支持发送请求并等待对应响应）。
type cdpClient struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	nextID int
}

type cdpMessage struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *cdpClient) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	msg := cdpMessage{ID: id, Method: method}
	if params != nil {
		b, _ := json.Marshal(params)
		msg.Params = b
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, err
	}

	for {
		var resp cdpMessage
		if err := c.conn.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.ID != id {
			continue // 事件或其他响应，跳过
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP 错误 %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// getAllCookies 读取浏览器当前全部 cookie。
func (c *cdpClient) getAllCookies() ([]cookie, error) {
	result, err := c.call("Network.getAllCookies", nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		Cookies []cookie `json:"cookies"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return nil, err
	}
	return r.Cookies, nil
}

type cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

// extractCookies 从浏览器 cookie 列表中提取完整会话。
// 仅当捕获到两个 passport cookie 时才认为登录完成，此时返回**全部** cookie
// （NGA 会话可能依赖多个 cookie，只存两个可能失效）。
func extractCookies(cookies []cookie) map[string]string {
	var uid, cid string
	out := map[string]string{}
	for _, c := range cookies {
		out[c.Name] = c.Value
		if c.Name == UidCookie {
			uid = c.Value
		}
		if c.Name == CidCookie {
			cid = c.Value
		}
	}
	if uid == "" || cid == "" {
		return nil
	}
	return out
}
