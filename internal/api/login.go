package api

import (
	"net/url"
	"strings"

	"github.com/ArkURL/nga-tui/internal/debug"
)

// Session 表示登录状态。
type Session struct {
	UID string
	// Cookies 是需要附加到 bbs.nga.cn 请求的 passport cookie。
	Cookies map[string]string
}

// CheckLogin 通过登录门槛接口判断当前 cookie 是否有效。
// 未登录时 NGA 明确返回 2048:尚未登录，返回 (false, nil)；
// 请求本身失败（网络/HTTP）时返回错误。
func (c *Client) CheckLogin() (bool, error) {
	params := url.Values{}
	params.Set("favor", "1") // 收藏主题，需登录
	params.Set("__output", "11")

	body, err := c.Get("/thread.php", params)
	if err != nil {
		debug.Logf("CheckLogin 请求失败: %v", err)
		return false, err
	}
	if _, err := parseRoot(body); err != nil {
		if ae, ok := err.(*APIError); ok && strings.Contains(ae.Code, "2048") {
			debug.Logf("CheckLogin: 会话未登录（2048），当前 cookie=%d 个", len(c.Cookies()))
			return false, nil // 明确未登录
		}
		debug.Logf("CheckLogin 解析异常: %v", err)
		return false, err
	}
	debug.Logf("CheckLogin: 会话有效")
	return true, nil
}
