package model

import (
	"encoding/json"
)

// SubForum 是父版面下的子版面（或合集），来自 thread.php 响应的 __F.sub_forums。
// IsCollection 为 true 时 ID 是 stid（需用 stid 参数打开），否则 ID 是 fid。
type SubForum struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Info         string `json:"info,omitempty"`
	IsCollection bool   `json:"is_collection,omitempty"`
}

// BoardRef 转收藏引用：合集存 STID，普通子版面存 FID。
func (s SubForum) BoardRef() BoardRef {
	br := BoardRef{Name: s.Name}
	if s.IsCollection {
		br.STID = s.ID
	} else {
		br.FID = s.ID
	}
	return br
}

// BoardRef 是收藏的版面引用（fid 或 stid + 名称），可指向分类树里的版面或任意子版面。
type BoardRef struct {
	FID  string `json:"fid,omitempty"`
	STID string `json:"stid,omitempty"`
	Name string `json:"name,omitempty"`
}

// Key 返回收藏键：stid 优先（合集），否则 fid。合集与子版面用同一个键体系。
func (b BoardRef) Key() string {
	if b.STID != "" {
		return b.STID
	}
	return b.FID
}

// UnmarshalJSON 兼容两种格式：
//   - 旧配置：裸字符串 fid（"7"）
//   - 新配置：{"fid":"7","stid":"","name":"..."}
func (b *BoardRef) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		b.FID = s
		return nil
	}
	type wire struct {
		FID  string `json:"fid"`
		STID string `json:"stid"`
		Name string `json:"name"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	b.FID = w.FID
	b.STID = w.STID
	b.Name = w.Name
	return nil
}
