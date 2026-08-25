package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ArkURL/nga-tui/internal/debug"
)

// 默认 User-Agent。NGA 对非移动端 UA 会返回反爬 error 15（访客不能直接访问），
// 使用移动端 UA 可绕过。
const DefaultUA = "NGA_WP_JW"

// BaseURL 为 NGA 主站。
const BaseURL = "https://bbs.nga.cn"

// Client 是 NGA 数据层核心：持有 UA、cookie 与请求限速。
type Client struct {
	// base 是主站地址，测试时可替换。
	base string
	// ua 用于全部请求。
	ua string
	// hc 是底层 http.Client。
	hc *http.Client

	// cookies 手动附加到请求（登录后从 account.178.com 取得的 passport cookie）。
	cookies map[string]string
	mu      sync.Mutex

	// onCookiesChanged 在会话 cookie 因跟随 Set-Cookie 而变化时被调用（用于持久化）。
	onCookiesChanged func()

	// 限速：两次请求最小间隔。
	minInterval time.Duration
	lastReq     time.Time
	rateMu      sync.Mutex
}

// NewClient 创建默认客户端。
func NewClient() *Client {
	return &Client{
		base:        BaseURL,
		ua:          DefaultUA,
		hc:          &http.Client{Timeout: 20 * time.Second},
		cookies:     map[string]string{},
		minInterval: 200 * time.Millisecond,
	}
}

// SetCookies 设置手动附加的 cookie（ngaPassportUid / ngaPassportCid 等）。
func (c *Client) SetCookies(cs map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cookies = cs
}

// Cookies 返回当前 cookie 副本，供持久化。
func (c *Client) Cookies() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.cookies))
	for k, v := range c.cookies {
		out[k] = v
	}
	return out
}

// LoggedIn 粗略判断是否带上了 passport cookie。
func (c *Client) LoggedIn() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cookies["ngaPassportUid"] != "" && c.cookies["ngaPassportCid"] != ""
}

// SetOnCookiesChanged 注册会话 cookie 变化回调（跟随 Set-Cookie 轮换后触发）。
func (c *Client) SetOnCookiesChanged(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onCookiesChanged = fn
}

// mergeResponseCookies 跟随服务端 Set-Cookie 更新会话 cookie。
// NGA 可能轮换 passport token，若不更新会因旧 token 失效导致会话丢失。
// 注意：只在拿到非空的会话 cookie 值时更新，绝不因"清空"指令删除，
// 避免错误响应误清已保存的会话。返回会话 cookie 是否有变化。
func (c *Client) mergeResponseCookies(h http.Header) bool {
	if len(h) == 0 {
		return false
	}
	changed := false
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sc := range h.Values("Set-Cookie") {
		name, value, ok := parseSetCookie(sc)
		if !ok {
			continue
		}
		// 只跟随会话相关 cookie，避免跟踪类 cookie 干扰
		if !strings.HasPrefix(name, "ngaPassport") {
			continue
		}
		if value == "" {
			// 空值（可能为删除指令）：不处理，保护现有会话
			continue
		}
		if c.cookies[name] != value {
			c.cookies[name] = value
			changed = true
		}
	}
	return changed
}

// parseSetCookie 从单个 Set-Cookie 头提取 name=value。
func parseSetCookie(header string) (name, value string, ok bool) {
	parts := strings.SplitN(header, ";", 2)
	if len(parts) == 0 {
		return "", "", false
	}
	kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(kv) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]), true
}

// Get 发起 GET 请求，返回原始响应体。
func (c *Client) Get(path string, params url.Values) ([]byte, error) {
	body, _, err := c.doHost("GET", c.base, path, params, nil)
	return body, err
}

// fetchJSON 获取并解析响应；解析失败（如 NGA 响应被截断）时退避重试。
// APIError/HTTPError 等确定性错误不重试。
func (c *Client) fetchJSON(path string, params url.Values, parse func(body []byte) error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, err := c.Get(path, params)
		if err != nil {
			return err
		}
		if err := parse(body); err != nil {
			if _, ok := err.(*APIError); ok {
				return err
			}
			if _, ok := err.(*HTTPError); ok {
				return err
			}
			debug.Logf("解析响应失败（第 %d 次）: %v", attempt+1, err)
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		return nil
	}
	return fmt.Errorf("多次尝试后解析仍失败: %w", lastErr)
}

func (c *Client) doHost(method, host, path string, params url.Values, body io.Reader) ([]byte, http.Header, error) {
	full := host + path
	if params != nil && len(params) > 0 {
		full += "?" + params.Encode()
	}

	req, err := http.NewRequest(method, full, body)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("User-Agent", c.ua)

	c.mu.Lock()
	for k, v := range c.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	c.mu.Unlock()

	c.throttle()
	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = c.hc.Do(req)
		if err == nil && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		// 429 / 5xx：退避重试
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Header, &HTTPError{Status: resp.StatusCode, URL: full}
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, err
	}
	// 跟随服务端轮换的会话 cookie，并通知持久化
	if c.mergeResponseCookies(resp.Header) {
		debug.Logf("会话 cookie 更新（跟随 Set-Cookie）")
		c.mu.Lock()
		fn := c.onCookiesChanged
		c.mu.Unlock()
		if fn != nil {
			fn()
		}
	}
	return buf, resp.Header, nil
}

// HTTPError 表示非 200 的 HTTP 响应。
type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string {
	switch e.Status {
	case 403:
		return "HTTP 403: 该版面需要登录后才能访问（按 L 登录）"
	case 404:
		return "HTTP 404: 资源不存在"
	case 429:
		return "HTTP 429: 请求过于频繁，请稍后再试"
	default:
		return fmt.Sprintf("HTTP %d", e.Status)
	}
}

// throttle 保证两次请求间隔不小于 minInterval。
func (c *Client) throttle() {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	if c.minInterval <= 0 {
		return
	}
	wait := c.minInterval - time.Since(c.lastReq)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
}
