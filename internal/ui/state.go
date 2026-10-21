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
	// Favorites 收藏的版面，按用户自定义顺序排列。
	// 用切片而非 map 是为了保留手动排序（Shift+J/K 调序）。
	Favorites []model.BoardRef

	// 帖子列表状态
	CurrentForum *model.Forum
	Threads      []model.Thread
	SubForums    []model.SubForum // 当前版面的子版面/合集
	// BoardStack 钻取栈：进入子版面时压入当前版面，返回时弹出。
	BoardStack  []model.Forum
	ListPage    int
	ListPages   int
	ListOrderBy string
	// ListSearchKey 非空表示当前列表是搜索结果。
	ListSearchKey string
	// ListReload 进入帖子列表视图时需要重新加载（切版面/新搜索时为 true；
	// 从阅读页返回时保持 false，避免刷新丢失选中项）。
	ListReload bool

	// 阅读状态
	CurrentThread *model.Thread
	Replies       []model.Reply
	ReplyUsers    map[int]model.User
	ReadPage      int
	ReadPages     int
}

// NewState 创建初始状态。
func NewState(client *api.Client) *State {
	return &State{
		Client:      client,
		ListOrderBy: "lastpostdesc",
	}
}

// IsFavorite 判断版面是否已收藏。
func (s *State) IsFavorite(key string) bool {
	return s.favoriteIndex(key) >= 0
}

// favoriteIndex 返回收藏在列表中的位置，不存在返回 -1。
func (s *State) favoriteIndex(key string) int {
	if key == "" {
		return -1
	}
	for i, ref := range s.Favorites {
		if ref.Key() == key {
			return i
		}
	}
	return -1
}

// AddFavorite 追加收藏；已存在时更新内容但保持原位置。
func (s *State) AddFavorite(ref model.BoardRef) {
	key := ref.Key()
	if key == "" {
		return
	}
	if i := s.favoriteIndex(key); i >= 0 {
		s.Favorites[i] = ref
		return
	}
	s.Favorites = append(s.Favorites, ref)
}

// RemoveFavorite 删除收藏，返回是否删除。
func (s *State) RemoveFavorite(key string) bool {
	i := s.favoriteIndex(key)
	if i < 0 {
		return false
	}
	s.Favorites = append(s.Favorites[:i], s.Favorites[i+1:]...)
	return true
}

// MoveFavorite 把 key 对应的收藏移动 delta 格（-1 上移 / 1 下移），
// 越界时不动并返回 false。
func (s *State) MoveFavorite(key string, delta int) bool {
	i := s.favoriteIndex(key)
	if i < 0 {
		return false
	}
	j := i + delta
	if j < 0 || j >= len(s.Favorites) {
		return false
	}
	s.Favorites[i], s.Favorites[j] = s.Favorites[j], s.Favorites[i]
	return true
}

// NavigateMsg 是视图间导航请求，由根 App 处理。
type NavigateMsg struct {
	Screen  Screen
	Payload any
}
