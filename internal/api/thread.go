package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

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
}

// GetThreads 拉取版面的帖子列表；keyword 非空时执行版内搜索。
func (c *Client) GetThreads(fid string, page int, orderBy, keyword string) (*ThreadListResult, error) {
	params := url.Values{}
	params.Set("fid", fid)
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
