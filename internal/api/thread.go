package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/ArkURL/nga-tui/internal/model"
)

// PageSize 是帖子列表的每页条数（NGA 移动接口固定 50）。
const PageSize = 50

// ReadPageSize 是帖子阅读接口的每页楼层数（read.php 的 __R__ROWS_PAGE，通常 20）。
const ReadPageSize = 20

// ThreadListResult 是帖子列表接口的解析结果。
type ThreadListResult struct {
	Threads []model.Thread
	Page    int
	Pages   int
	OrderBy string
	// SubForums 当前版面的子版面/合集（来自 __F.sub_forums）。
	SubForums []model.SubForum
}

// GetThreads 拉取版面的帖子列表；keyword 非空时执行版内搜索。
// stid 非空时按合集打开（thread.php?stid=...），否则按 fid 打开。
func (c *Client) GetThreads(fid, stid string, page int, orderBy, keyword string) (*ThreadListResult, error) {
	params := url.Values{}
	if stid != "" {
		params.Set("stid", stid)
	} else {
		params.Set("fid", fid)
	}
	if page > 1 {
		params.Set("page", strconv.Itoa(page))
	}
	if keyword != "" {
		params.Set("key", keyword)
	}
	if orderBy != "" {
		params.Set("order_by", orderBy)
	}
	params.Set("__output", "11")

	res := &ThreadListResult{OrderBy: orderBy}
	err := c.fetchJSON("/thread.php", params, func(body []byte) error {
		data, err := parseData(body)
		if err != nil {
			return err
		}
		res.Page = intFromRaw(data["__PAGE"])
		if res.Page == 0 {
			res.Page = page
		}
		if rows := intFromRaw(data["__ROWS"]); rows > 0 {
			res.Pages = (rows + PageSize - 1) / PageSize
		}
		// __T 是数组（保持服务端顺序，置顶帖在前）
		if rawT, ok := data["__T"]; ok {
			if err := json.Unmarshal(rawT, &res.Threads); err != nil {
				return fmt.Errorf("解析帖子列表 __T: %w", err)
			}
		}
		// __F.sub_forums 是当前版面的子版面/合集
		if fRaw, ok := data["__F"]; ok {
			var fmap map[string]json.RawMessage
			if json.Unmarshal(fRaw, &fmap) == nil {
				res.SubForums = parseSubForums(fmap["sub_forums"])
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// parseData 解码响应，下钻到 data 层并检查错误包络。
func parseData(body []byte) (map[string]json.RawMessage, error) {
	root, err := parseRoot(body)
	if err != nil {
		return nil, err
	}
	if rawData, ok := root["data"]; ok {
		var data map[string]json.RawMessage
		if err := json.Unmarshal(rawData, &data); err != nil {
			return nil, fmt.Errorf("解析 data 层: %w", err)
		}
		return data, nil
	}
	return root, nil
}

// parseRoot 解码响应外层为 map，并检查 NGA 错误包络。
func parseRoot(body []byte) (map[string]json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(fixJSON(body), &root); err != nil {
		return nil, fmt.Errorf("解析响应: %w", err)
	}
	if rawErr, ok := root["error"]; ok {
		var errList []string
		if json.Unmarshal(rawErr, &errList) == nil && len(errList) > 0 {
			return nil, &APIError{Code: errList[0]}
		}
	}
	return root, nil
}

// intFromRaw 从 json.RawMessage 提取 int，失败返回 0。
func intFromRaw(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0
	}
	v, _ := strconv.Atoi(n.String())
	return v
}

// strFromRaw 从 json.RawMessage 提取字符串，兼容数字与字符串。
func strFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		return ""
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// parseSubForums 解析 __F.sub_forums（可为 null/缺失）。
// 格式：{"<fid>":[fid,name,info,?,type],"t<stid>":[stid,name,info,stid,type]}
// 键前缀 "t" 表示合集（用 stid 打开），否则是普通子版面（用 fid 打开）。
func parseSubForums(raw json.RawMessage) []model.SubForum {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	out := make([]model.SubForum, 0, len(m))
	for key, val := range m {
		var arr []json.RawMessage
		if json.Unmarshal(val, &arr) != nil || len(arr) < 2 {
			continue
		}
		out = append(out, model.SubForum{
			ID:           strFromRaw(arr[0]),
			Name:         strFromRaw(arr[1]),
			Info:         strFromRaw(safeAt(arr, 2)),
			IsCollection: strings.HasPrefix(key, "t"),
		})
	}
	// 排序：数字 id 优先（保持确定性）
	sort.Slice(out, func(i, j int) bool {
		ii, ei := strconv.Atoi(out[i].ID)
		jj, ej := strconv.Atoi(out[j].ID)
		if ei == nil && ej == nil && ii != jj {
			return ii < jj
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// safeAt 返回切片第 i 个元素，越界返回空。
func safeAt(arr []json.RawMessage, i int) json.RawMessage {
	if i < len(arr) {
		return arr[i]
	}
	return nil
}
