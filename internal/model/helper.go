package model

import (
	"encoding/json"
	"strconv"
	"strings"
)

// rawInt 把 json.RawMessage 转为 int，兼容数字、字符串数字、null 等。
// NGA 返回的 JSON 字段类型不固定（数字或字符串混用），统一容错。
func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	s := strings.TrimSpace(string(raw))
	if s == "null" || s == `""` {
		return 0
	}
	if s[0] == '"' {
		var str string
		if json.Unmarshal(raw, &str) != nil {
			return 0
		}
		str = strings.TrimSpace(str)
		if str == "" {
			return 0
		}
		n, err := strconv.Atoi(str)
		if err != nil {
			return 0 // 无法解析的字符串（如匿名标识）按 0 处理
		}
		return n
	}
	var n int
	if json.Unmarshal(raw, &n) != nil {
		return 0
	}
	return n
}

// rawBool 把 json.RawMessage 转为 bool，兼容 true/false/1/0 及其字符串形式。
func rawBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	s := strings.TrimSpace(string(raw))
	s = strings.Trim(s, `"`)
	return s == "true" || s == "1"
}
