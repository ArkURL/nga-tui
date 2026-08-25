package model

import (
	"encoding/json"
	"testing"
)

// 模拟 NGA 类型不严谨：数字字段有时是字符串。
func TestThreadStringNumbers(t *testing.T) {
	src := `{"tid":"12345","fid":7,"subject":"标题","author":"张三","authorid":"64371487","postdate":"1787612286","lastpost":1787619716,"replies":"5","type":0}`
	var th Thread
	if err := json.Unmarshal([]byte(src), &th); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if th.TID != 12345 || th.FID != 7 || th.AuthorID != 64371487 {
		t.Fatalf("容错解析结果不对: %+v", th)
	}
	if th.PostDate != 1787612286 || th.LastPost != 1787619716 || th.Replies != 5 {
		t.Fatalf("容错解析结果不对: %+v", th)
	}
}

func TestThreadNullAndBool(t *testing.T) {
	src := `{"tid":1,"subject":"x","authorid":null,"if_lock":"1","is_poll":0}`
	var th Thread
	if err := json.Unmarshal([]byte(src), &th); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if th.AuthorID != 0 || !th.IsLock || th.IsPoll {
		t.Fatalf("null/bool 容错解析不对: %+v", th)
	}
}

func TestReplyStringNumbers(t *testing.T) {
	src := `{"pid":"123","tid":456,"lou":"3","authorid":"42","content":"内容","postdate":"2026-08-17 16:26","postdatetimestamp":"1787612286"}`
	var rp Reply
	if err := json.Unmarshal([]byte(src), &rp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if rp.PID != 123 || rp.TID != 456 || rp.Lou != 3 || rp.AuthorID != 42 {
		t.Fatalf("容错解析结果不对: %+v", rp)
	}
	if rp.PostDate != "2026-08-17 16:26" || rp.PostDatetime != 1787612286 {
		t.Fatalf("时间字段解析不对: %+v", rp)
	}
}

func TestUserStringUID(t *testing.T) {
	src := `{"uid":"64371487","username":"UID:64371487","avatar":null}`
	var u User
	if err := json.Unmarshal([]byte(src), &u); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if u.UID != 64371487 || u.Avatar != "" {
		t.Fatalf("容错解析结果不对: %+v", u)
	}
}
