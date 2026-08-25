package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ArkURL/nga-tui/internal/model"
)

// ThreadContentResult 是 read.php 的解析结果。
type ThreadContentResult struct {
	Replies []model.Reply  // 楼层，按服务端顺序（主楼在前）
	Users   map[int]model.User
	Page    int
	Pages   int
	Rows    int // 总楼层数（含主楼）
}

// GetThreadContent 拉取帖子某一页的内容。
func (c *Client) GetThreadContent(tid int, page int) (*ThreadContentResult, error) {
	params := url.Values{}
	params.Set("tid", strconv.Itoa(tid))
	if page > 1 {
		params.Set("page", strconv.Itoa(page))
	}
	params.Set("__output", "11")

	body, err := c.Get("/read.php", params)
	if err != nil {
		return nil, err
	}

	data, err := parseData(body)
	if err != nil {
		return nil, err
	}

	res := &ThreadContentResult{Page: intFromRaw(data["__PAGE"]), Rows: intFromRaw(data["__ROWS"])}
	if res.Page == 0 {
		res.Page = page
	}
	// 优先用 __R__ROWS_PAGE 作为每页楼层数，否则按 ReadPageSize
	pageSize := intFromRaw(data["__R__ROWS_PAGE"])
	if pageSize <= 0 {
		pageSize = ReadPageSize
	}
	if res.Rows > 0 {
		res.Pages = (res.Rows + pageSize - 1) / pageSize
	}

	// __R 是数组（保持楼层顺序）
	if rawR, ok := data["__R"]; ok {
		if err := json.Unmarshal(rawR, &res.Replies); err != nil {
			return nil, fmt.Errorf("解析楼层 __R: %w", err)
		}
	}

	// __U 是 map，key 为 uid，但混入了 __GROUPS/__MEDALS 等辅助数据，
	// 只取纯数字 key 的项，其余跳过。
	if rawU, ok := data["__U"]; ok {
		res.Users = map[int]model.User{}
		var umap map[string]json.RawMessage
		if err := json.Unmarshal(rawU, &umap); err != nil {
			return nil, fmt.Errorf("解析用户 __U: %w", err)
		}
		for k, raw := range umap {
			if !allDigits(k) {
				continue
			}
			var u model.User
			if err := json.Unmarshal(raw, &u); err != nil {
				continue // 单项失败跳过，不拖垮整页
			}
			uid, _ := strconv.Atoi(k)
			res.Users[uid] = u
		}
	}
	return res, nil
}

// allDigits 判断字符串是否全部由数字组成。
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
