package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestEscapeControlChars(t *testing.T) {
	// alterinfo 内含原始制表符（真实 \t 字符）
	in := []byte(`{"alterinfo":" 2016-03-12 00:01:00  版主	修改"}`)
	out := escapeControlChars(in)
	var m map[string]string
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("解析失败: %v\n原始输出: %q", err, out)
	}
	if !strings.Contains(m["alterinfo"], "\t") {
		t.Fatalf("期望保留 \t，得到 %q", m["alterinfo"])
	}
}

func TestQuoteNumberKeys(t *testing.T) {
	in := []byte(`{"__T":{"123":{"tid":123,"subject":"x"},"456":{"tid":456}},"__PAGE":2}`)
	out := quoteNumberKeys(in)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("解析失败: %v\n输出: %s", err, out)
	}
	var tmap map[string]json.RawMessage
	if err := json.Unmarshal(m["__T"], &tmap); err != nil {
		t.Fatalf("解析 __T 失败: %v", err)
	}
	if _, ok := tmap["123"]; !ok {
		t.Fatalf("缺少 key 123: %s", out)
	}
	if _, ok := tmap["456"]; !ok {
		t.Fatalf("缺少 key 456: %s", out)
	}
}

func TestFixJSON(t *testing.T) {
	// 组合场景：数字 key + 字符串内 \t + HTML 实体
	in := []byte(`{"__R":{"0":{"content":"第一楼\t内容","pid":0},"17":{"content":"quote [b]加粗[/b]","pid":17}},"__U":{"5":{"username":"测试"}}}`)
	out := fixJSON(in)
	if err := json.Valid(out); !err {
		t.Fatalf("fixJSON 输出非法: %s", out)
	}
}

func TestDecodeResponseErrorEnvelope(t *testing.T) {
	body := []byte(`{"error":["15:访客不能直接访问","<html>...</html>",null],"data":{"__MESSAGE":["游客"]}}`)
	var v any
	err := decodeResponse(body, &v)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("期望 *APIError，得到 %T: %v", err, err)
	}
	if !strings.Contains(apiErr.Code, "15") {
		t.Fatalf("期望含 15，得到 %q", apiErr.Code)
	}
}

func TestDecodeResponseDataDrillDown(t *testing.T) {
	body := []byte(`{"data":[{"id":1,"name":"网事杂谈"}]}`)
	var cats []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := decodeResponse(body, &cats); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != "网事杂谈" {
		t.Fatalf("解码结果不对: %+v", cats)
	}
}

func TestFixJSONPreservesEscapes(t *testing.T) {
	// 已转义的 \\t 不应被再次处理
	in := []byte(`{"content":"line1\\tline2"}`)
	out := escapeControlChars(in)
	if string(out) != string(in) {
		t.Fatalf("不应修改已转义内容: %q", out)
	}
}

func TestMergeResponseCookies(t *testing.T) {
	c := NewClient()
	c.SetCookies(map[string]string{
		"ngaPassportUid": "12345",
		"ngaPassportCid": "old-token",
	})

	h := http.Header{}
	h.Add("Set-Cookie", "ngaPassportCid=new-token; Path=/; HttpOnly")
	h.Add("Set-Cookie", "_ga=GA1.1; Path=/") // 跟踪 cookie 不应跟随

	changed := c.mergeResponseCookies(h)
	if !changed {
		t.Fatal("会话 cookie 变化应返回 true")
	}
	got := c.Cookies()
	if got["ngaPassportCid"] != "new-token" {
		t.Fatalf("应更新 cid，得到 %+v", got)
	}
	if got["ngaPassportUid"] != "12345" {
		t.Fatalf("uid 不应变化: %+v", got)
	}
	if _, ok := got["_ga"]; ok {
		t.Fatalf("非会话 cookie 不应跟随: %+v", got)
	}
}

func TestMergeResponseCookiesNoChange(t *testing.T) {
	c := NewClient()
	c.SetCookies(map[string]string{"ngaPassportUid": "1", "ngaPassportCid": "same"})
	h := http.Header{}
	h.Add("Set-Cookie", "ngaPassportCid=same; Path=/")
	if c.mergeResponseCookies(h) {
		t.Fatal("值相同不应视为变化")
	}
}

func TestMergeResponseCookiesGuestNotFollow(t *testing.T) {
	// 未登录（无 uid）时不应跟随 Set-Cookie，避免写入垃圾 token
	c := NewClient()
	c.SetCookies(map[string]string{"ngaPassportCid": "guest-token"})
	h := http.Header{}
	h.Add("Set-Cookie", "ngaPassportCid=some-token; Path=/")
	if c.mergeResponseCookies(h) {
		t.Fatal("访客会话不应跟随 Set-Cookie")
	}
	if c.Cookies()["ngaPassportCid"] != "guest-token" {
		t.Fatal("访客 cookie 不应被修改")
	}
}

func TestParseSetCookie(t *testing.T) {
	name, value, ok := parseSetCookie("ngaPassportCid=abc; Path=/; HttpOnly")
	if !ok || name != "ngaPassportCid" || value != "abc" {
		t.Fatalf("解析失败: %q %q %v", name, value, ok)
	}
}

func TestClientFollowsSetCookieEndToEnd(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Cookie"))
		mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "ngaPassportCid", Value: "token-v2", Path: "/"})
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := NewClient()
	c.base = ts.URL
	c.minInterval = 0
	c.SetCookies(map[string]string{"ngaPassportUid": "1", "ngaPassportCid": "token-v1"})

	if _, err := c.Get("/a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get("/b", nil); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) < 2 {
		t.Fatalf("请求次数不足: %v", seen)
	}
	if !strings.Contains(seen[0], "ngaPassportCid=token-v1") {
		t.Fatalf("首次请求应带旧 token: %v", seen[0])
	}
	if !strings.Contains(seen[1], "ngaPassportCid=token-v2") {
		t.Fatalf("第二次请求应带轮换后的 token: %v", seen[1])
	}
}

func TestEscapeControlCharsStrayBackslash(t *testing.T) {
	// 孤立反斜杠 \u 后跟非十六进制（非法转义）应被修复为字面量
	in := []byte(`{"content":"路径 C:\u 前面 和 文本"}`)
	out := escapeControlChars(in)
	var m map[string]string
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("解析失败: %v\n输出: %q", err, out)
	}
	if !strings.Contains(m["content"], `\u`) {
		t.Fatalf("应保留字面量 \\u: %q", m["content"])
	}
}

func TestEscapeControlCharsValidUnicode(t *testing.T) {
	// 合法 \uXXXX 转义必须原样保留
	in := []byte(`{"content":"\u4e2d\u6587\\n测试"}`)
	out := escapeControlChars(in)
	var m map[string]string
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("解析失败: %v\n输出: %q", err, out)
	}
	if m["content"] != "中文\\n测试" {
		t.Fatalf("内容不对: %q", m["content"])
	}
}
