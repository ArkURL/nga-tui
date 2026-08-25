package ui

import (
	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/model"
)

// Screen 标识当前显示的视图。
type Screen int

const (
	ScreenForum Screen = iota
	ScreenThreadList
	ScreenReader
	ScreenSearch
	ScreenLogin
	ScreenHelp
)

func (s Screen) String() string {
	switch s {
	case ScreenForum:
		return "版面"
	case ScreenThreadList:
		return "帖子"
	case ScreenReader:
		return "阅读"
	case ScreenSearch:
		return "搜索"
	case ScreenLogin:
		return "登录"
	case ScreenHelp:
		return "帮助"
	}
	return "?"
}

// State 是跨视图共享的会话状态。
type State struct {
	Client *api.Client
	// LoggedIn 是否已带登录 cookie。
	LoggedIn bool

	Categories []model.Category
	// Favorites 收藏的版面 fid。
	Favorites map[string]bool

	// 帖子列表状态
	CurrentForum *model.Forum
	Threads      []model.Thread
	ListPage     int
	ListPages    int
	ListOrderBy  string
	// ListSearchKey 非空表示当前列表是搜索结果。
	ListSearchKey string
	// ListReload 进入帖子列表视图时需要重新加载（切版面/新搜索时为 true；
	// 从阅读页返回时保持 false，避免刷新丢失选中项）。
	ListReload bool

	// 阅读状态
	CurrentThread *model.Thread
	Replies      []model.Reply
	ReplyUsers   map[int]model.User
	ReadPage     int
	ReadPages    int
}

// NewState 创建初始状态。
func NewState(client *api.Client) *State {
	return &State{
		Client:      client,
		ListOrderBy: "lastpostdesc",
		Favorites:   map[string]bool{},
	}
}

// NavigateMsg 是视图间导航请求，由根 App 处理。
type NavigateMsg struct {
	Screen  Screen
	Payload any
}
