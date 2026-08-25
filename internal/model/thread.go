package model

import "encoding/json"

// Thread 是帖子列表中的一篇主题，对应 thread.php 响应 __T 数组中的项。
// 用自定义 UnmarshalJSON 兼容 NGA 数字/字符串混用的字段类型。
type Thread struct {
	TID        int    `json:"tid"`
	FID        int    `json:"fid"`
	Subject    string `json:"subject"`
	Author     string `json:"author"`
	AuthorID   int    `json:"authorid"`
	PostDate   int64  `json:"postdate"`
	LastPost   int64  `json:"lastpost"`
	LastPoster string `json:"lastposter"`
	Replies    int    `json:"replies"`
	Type       int    `json:"type"`
	IsLock     bool   `json:"if_lock"`
	IsPoll     bool   `json:"is_poll"`
	IsAnnounce bool   `json:"is_announce"`
}

// UnmarshalJSON 对可能为数字或字符串的字段做容错转换。
func (t *Thread) UnmarshalJSON(data []byte) error {
	type wire struct {
		TID        json.RawMessage `json:"tid"`
		FID        json.RawMessage `json:"fid"`
		Subject    string          `json:"subject"`
		Author     string          `json:"author"`
		AuthorID   json.RawMessage `json:"authorid"`
		PostDate   json.RawMessage `json:"postdate"`
		LastPost   json.RawMessage `json:"lastpost"`
		LastPoster string          `json:"lastposter"`
		Replies    json.RawMessage `json:"replies"`
		Type       json.RawMessage `json:"type"`
		IsLock     json.RawMessage `json:"if_lock"`
		IsPoll     json.RawMessage `json:"is_poll"`
		IsAnnounce json.RawMessage `json:"is_announce"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	t.Subject = w.Subject
	t.Author = w.Author
	t.LastPoster = w.LastPoster
	t.TID = rawInt(w.TID)
	t.FID = rawInt(w.FID)
	t.AuthorID = rawInt(w.AuthorID)
	t.PostDate = int64(rawInt(w.PostDate))
	t.LastPost = int64(rawInt(w.LastPost))
	t.Replies = rawInt(w.Replies)
	t.Type = rawInt(w.Type)
	t.IsLock = rawBool(w.IsLock)
	t.IsPoll = rawBool(w.IsPoll)
	t.IsAnnounce = rawBool(w.IsAnnounce)
	return nil
}
