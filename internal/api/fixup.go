package api

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// FixJSON 导出预处理函数，供调试/测试使用。
func FixJSON(data []byte) []byte { return fixJSON(data) }

// fixJSON 对 NGA 返回的不严谨 JSON 做预处理，返回标准 JSON。
// NGA 的 JSON 常见问题：
//  1. 对象 key 是裸数字（如 {"__R":{"123":{...}}}），未加引号
//  2. 字符串值内包含原始控制字符（如 alterinfo 里的 \t），破坏 encoding/json 解析
//
// 两道处理都用状态机追踪 inString/escaped，只修改字符串外的结构，避免误伤字符串内容。
func fixJSON(data []byte) []byte {
	data = escapeControlChars(data)
	data = quoteNumberKeys(data)
	return data
}

// escapeControlChars 把字符串值内的控制字符（< 0x20）改写为合法转义序列，
// 并修复孤立反斜杠（后面不是合法转义的 `\` 转义为 `\\`，避免解析失败）。
func escapeControlChars(data []byte) []byte {
	var buf bytes.Buffer
	inStr, escaped := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inStr {
			switch {
			case escaped:
				buf.WriteByte(c)
				escaped = false
			case c == '\\':
				if isValidJSONEscape(data, i) {
					buf.WriteByte(c)
					escaped = true
				} else {
					// 孤立反斜杠：转义自身，后续字符按字面量处理
					buf.WriteString(`\\`)
				}
			case c == '"':
				buf.WriteByte(c)
				inStr = false
			case c < 0x20:
				switch c {
				case '\n':
					buf.WriteString(`\n`)
				case '\r':
					buf.WriteString(`\r`)
				case '\t':
					buf.WriteString(`\t`)
				case '\b':
					buf.WriteString(`\b`)
				case '\f':
					buf.WriteString(`\f`)
				default:
					fmt.Fprintf(&buf, `\u%04x`, c)
				}
			default:
				buf.WriteByte(c)
			}
			continue
		}
		if c == '"' {
			inStr = true
		}
		buf.WriteByte(c)
	}
	return buf.Bytes()
}

// isValidJSONEscape 判断 data[i]（反斜杠）是否为合法的 JSON 转义开头。
func isValidJSONEscape(data []byte, i int) bool {
	if i+1 >= len(data) {
		return false
	}
	switch data[i+1] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return true
	case 'u':
		// 需要 4 个十六进制位
		if i+6 > len(data) {
			return false
		}
		for j := i + 2; j < i+6; j++ {
			if !isHex(data[j]) {
				return false
			}
		}
		return true
	}
	return false
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// quoteNumberKeys 给对象中裸数字 key 补引号：{ 或 , 之后，空白 + 数字串 + 空白 + : 的形式。
func quoteNumberKeys(data []byte) []byte {
	var buf bytes.Buffer
	inStr, escaped := false, false
	for i := 0; i < len(data); {
		c := data[i]
		if inStr {
			buf.WriteByte(c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inStr = false
			}
			i++
			continue
		}
		if c == '"' {
			inStr = true
			buf.WriteByte(c)
			i++
			continue
		}
		if c == '{' || c == ',' {
			buf.WriteByte(c)
			j := skipSpace(data, i+1)
			if j < len(data) && isDigit(data[j]) {
				k := j
				for k < len(data) && isDigit(data[k]) {
					k++
				}
				m := skipSpace(data, k)
				if m < len(data) && data[m] == ':' {
					buf.WriteByte('"')
					buf.Write(data[j:k])
					buf.WriteString(`":`)
					i = m + 1
					continue
				}
			}
			i++
			continue
		}
		buf.WriteByte(c)
		i++
	}
	return buf.Bytes()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func skipSpace(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	return i
}

// decodeResponse 解码 NGA 响应体。部分接口返回 {"data":{...}}，部分直接返回对象，
// 统一先解 map[string]json.RawMessage，有 data 键则下钻一层。
func decodeResponse(body []byte, v any) error {
	body = fixJSON(body)

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查 error 包络
	if rawErr, ok := root["error"]; ok {
		var errList []string
		if err := json.Unmarshal(rawErr, &errList); err == nil && len(errList) > 0 {
			return &APIError{Code: errList[0]}
		}
	}

	if rawData, ok := root["data"]; ok {
		return json.Unmarshal(rawData, v)
	}
	return json.Unmarshal(body, v)
}

// APIError 表示 NGA 返回的业务错误，Code 形如 "15:访客不能直接访问"。
type APIError struct {
	Code string
}

func (e *APIError) Error() string { return "NGA 返回错误: " + e.Code }
