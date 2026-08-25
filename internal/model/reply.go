package model

import "encoding/json"

// Reply 是一条楼层回复，对应 read.php 响应 __R 数组中的项。pid=0 表示主楼。
// 注意：author 字段在 __R 中不存在，作者信息在 __U 中按 authorid 索引。
// 用自定义 UnmarshalJSON 兼容 NGA 数字/字符串混用的字段类型。
type Reply struct {
	PID          int    `json:"pid"`
	TID          int    `json:"tid"`
	Lou          int    `json:"lou"`
	Author       string `json:"author"`
	AuthorID     int    `json:"authorid"`
	Content      string `json:"content"`
	PostDate     string `json:"postdate"`          // 格式化时间，如 "2026-08-17 16:26"
	PostDatetime int64  `json:"postdatetimestamp"` // 时间戳（秒）
	AlterInfo    string `json:"alterinfo"`         // 编辑/操作记录，可能含原始制表符
}

// UnmarshalJSON 对可能为数字或字符串的字段做容错转换。
func (r *Reply) UnmarshalJSON(data []byte) error {
	type wire struct {
		PID          json.RawMessage `json:"pid"`
		TID          json.RawMessage `json:"tid"`
		Lou          json.RawMessage `json:"lou"`
		Author       string          `json:"author"`
		AuthorID     json.RawMessage `json:"authorid"`
		Content      string          `json:"content"`
		PostDate     json.RawMessage `json:"postdate"`
		PostDatetime json.RawMessage `json:"postdatetimestamp"`
		AlterInfo    string          `json:"alterinfo"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	r.Author = w.Author
	r.Content = w.Content
	r.AlterInfo = w.AlterInfo
	r.PID = rawInt(w.PID)
	r.TID = rawInt(w.TID)
	r.Lou = rawInt(w.Lou)
	r.AuthorID = rawInt(w.AuthorID)
	r.PostDate = rawToString(w.PostDate)
	r.PostDatetime = int64(rawInt(w.PostDatetime))
	return nil
}

// User 是用户信息，对应 read.php 响应 __U 中的项。
type User struct {
	UID      int    `json:"uid"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Group    string `json:"group"`
}

// UnmarshalJSON 对可能为数字或字符串的字段做容错转换。
func (u *User) UnmarshalJSON(data []byte) error {
	type wire struct {
		UID      json.RawMessage `json:"uid"`
		Username string          `json:"username"`
		Avatar   json.RawMessage `json:"avatar"`
		Group    string          `json:"group"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	u.Username = w.Username
	u.Group = w.Group
	u.UID = rawInt(w.UID)
	u.Avatar = rawToString(w.Avatar)
	return nil
}
