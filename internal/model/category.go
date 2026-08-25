package model

import (
	"encoding/json"
	"fmt"
)

// Category 是版面分类的顶级结构，对应 app_api.php category 接口的 data 数组元素。
type Category struct {
	ID     int     `json:"id"`
	IDStr  string  `json:"_id"`
	Name   string  `json:"name"`
	Groups []Group `json:"groups"`
}

// Group 是分类下的二级分组。
type Group struct {
	Name   string  `json:"name"`
	ID     int     `json:"id"`
	Info   string  `json:"info"`
	Forums []Forum `json:"forums"`
}

// Forum 是具体版面。注意 fid 在接口中类型不固定（int 或 string），
// 因此用自定义 UnmarshalJSON 兼容两种类型，统一存为 string。
type Forum struct {
	FID    string `json:"fid"`
	Name   string `json:"name"`
	Info   string `json:"info"`
	STID   string `json:"stid"` // 虚拟子版面（合集主题）
	IsList bool   `json:"is_forumlist"`
}

// UnmarshalJSON 兼容 fid/stid 为数字或字符串两种形式。
func (f *Forum) UnmarshalJSON(data []byte) error {
	type wire struct {
		FID    json.RawMessage `json:"fid"`
		STID   json.RawMessage `json:"stid"`
		Name   string          `json:"name"`
		Info   string          `json:"info"`
		IsList bool            `json:"is_forumlist"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	f.Name = w.Name
	f.Info = w.Info
	f.IsList = w.IsList
	f.FID = rawToString(w.FID)
	f.STID = rawToString(w.STID)
	return nil
}

// rawToString 把 json.RawMessage 转为字符串，兼容数字、字符串、空值。
func rawToString(raw json.RawMessage) string {
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
	return fmt.Sprintf("%s", raw)
}
