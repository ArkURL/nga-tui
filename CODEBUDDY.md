# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## 项目概述

NGA 论坛的终端客户端（TUI），Go + Bubble Tea。浏览版面/帖子、BBCode 渲染、登录（浏览器 CDP 抓 Cookie 或手动粘贴）。界面与代码注释均为中文。

## 常用命令

```bash
go run .                          # 本地运行
go build ./...                    # 编译
go test ./...                     # 全部单元测试（CI 使用 -timeout 120s）
go test ./internal/ui -run TestName -v   # 单个测试
go vet ./...                      # 静态检查（CI 必过）
gofmt -w .                        # 格式化
go run ./cmd/verify               # 实网探测一批版面的 sub_forums
go run ./cmd/verify <tid>         # 实网抓取并渲染指定帖子的 BBCode
```

工具链版本以 `go.mod` 为准（当前 `go 1.26.5`；README 里的 "Go 1.22+" 已过时）。

发布：推 `v*` tag 触发 GoReleaser（linux/darwin/windows × amd64/arm64）。CI 只在 push/PR 到 `main` 时跑 `go vet` + `go test`。

## 架构

数据流是单向的：视图发 `tea.Cmd` → 异步请求 → 返回自定义 Msg → 视图更新 → 发 `NavigateMsg` → 根 `App` 切换 screen 并初始化目标视图。

```
main.go                    读配置、恢复 cookie、CheckLogin、启动 tea.Program
└── internal/ui
    ├── app.go             根 model：screen 路由、全局按键、底栏、登录/登出、配置持久化
    ├── state.go           Screen 枚举、跨视图共享 State、NavigateMsg
    ├── keymap.go          bubbles/key 全局键位表
    ├── common.go          keyMatches/truncateLine/statusMsg/formatTime/isLoginError
    ├── forum.go           版面列表（分类树 + 收藏，本地构建，无分页）
    ├── threadlist.go      帖子列表（分页、排序、版内搜索、子版面切换）
    ├── reader.go          帖子阅读（viewport、翻页、按楼跳转）
    ├── search.go          三种 scope：搜版面 / 版内搜帖 / 链接或 fid-stid 直达
    ├── login.go           登录方式选择、浏览器抓取、手动粘贴 Cookie
    └── help.go            帮助页
internal/api               NGA 数据层：client / category / thread / read / login / fixup
internal/bbcode            BBCode → ANSI（手写解析树 + lipgloss 样式 + CJK 宽度换行）
                           NGA 表情 [s:xxx] 是自闭合的，解析时按文本处理并渲染为 〔xxx〕
internal/browser           Chrome DevTools Protocol 启动独立 Chrome 并抓 passport cookie
internal/model             领域模型，均为 NGA 脏 JSON 的容错反序列化
internal/config            ~/.config/nga-tui/config.json 原子读写
internal/debug             ~/.config/nga-tui/debug.log 诊断日志
cmd/verify                 开发辅助 CLI，直接打真实 NGA 接口
```

### 路由与视图约定

- `App.Update` 先处理全局键（`Ctrl+C`、`?`、`L`、根级 `q`），再把其余消息 `route` 给当前 screen。
- `ScreenLogin` / `ScreenSearch` 是文本输入视图：除 `Ctrl+C` 外全部按键直接放行，否则 `q`/`L` 会被全局拦截无法输入。这两个视图用 `Esc` 返回。
- 视图通过 `navCmd(screen, payload)` 发 `NavigateMsg`；`App.handleNavigate` 负责设置 `State.CurrentForum`/`CurrentThread`、重置分页，并在导航后补发 `WindowSizeMsg` 让新 viewport 正确初始化。
- 返回列表/阅读页时默认不刷新（保留选中项），只有切换版面或新搜索才置 `State.ListReload = true`。
- 每个视图自带 spinner + viewport；viewport 高度 = 窗口高度 - `footerHeight`(2) - 标题 1 行。`App.View` 统一拼接底栏与瞬态状态提示。
- 值接收者 `Init()` 无法写回字段，所以各列表视图另有指针方法 `start()` 供 App 导航时重置加载态。
- 布局是单栏、vim 风格键位（j/k/h/l/g/G、n/p 翻页、`/` 搜索、`f` 收藏、Tab 切换全部/收藏）。

### 异步响应防串台

列表/阅读请求的 `tea.Cmd` 会把请求参数（fid/stid/key/tid/page）一起带回 Msg，Update 时与当前 State 比对，不一致就丢弃。新增数据请求时保持同样模式，否则快速切版面会出现旧数据覆盖。

### NGA 数据层要点

- 所有请求走 `https://bbs.nga.cn` + `__output=11`；UA 必须是移动端 `NGA_WP_JW`，否则触发反爬 error 15。
- 接口：`/app_api.php?__lib=home&__act=category`（分类树）、`/thread.php`（帖子列表 + 子版面）、`/read.php`（楼层）、`/thread.php?favor=1`（登录探测）。
- `fixJSON` 是必需环节：NGA 会返回裸数字 key 和字符串内的原始控制字符/孤立反斜杠。任何新解析路径都要先走 `fetchJSON`。
- `fetchJSON` 对解析失败做 5 次指数退避重试（1/2/4/8s），但 `*APIError`/`*HTTPError` 是确定性错误不重试。解析失败会把 `body` 长度、`path`、`content-type` 写进 debug.log（空 body 记为「空响应」），排查服务端截断时先看这里。
- `Client` 内置 200ms 最小请求间隔和 429/5xx 重试。不要绕过 `throttle` 直接发请求。
- 已登录时会跟随 `Set-Cookie` 轮换 `ngaPassport*` 并回调持久化；空值/删除指令一律忽略，避免误清会话。
- 错误码 `2048`（"尚未登录"/"必须登录"）由 `isLoginError` 识别，视图会顺手把 `State.LoggedIn` 置 false。
- `model.Thread`/`Reply`/`User`/`Forum` 都用自定义 `UnmarshalJSON` 兼容数字与字符串混用；`__U` 里混有 `__GROUPS` 等非用户 key，只取纯数字 key。

### 持久化与副作用

- 配置 `~/.config/nga-tui/config.json`（0600）：cookies + favorites。写入走临时文件 rename，并由包级 mutex 串行化。
- `ui.persistAll` 有登录态保护：若待写入 cookie 不含 passport 而磁盘上已有，则保留旧 cookie，防止收藏操作或访客实例清掉登录。显式登出才用 `persistAllRaw`。
- 收藏键为 `BoardRef.Key()`：合集用 `stid`，普通版面用 `fid`。已有收藏时启动直接进入收藏视图。
- 收藏是**有序切片** `State.Favorites`（不是 map），配置中的顺序即显示顺序；收藏视图用 `Shift+J/K` 调序后立即持久化。增删/查询用 `State.IsFavorite/AddFavorite/RemoveFavorite/MoveFavorite`，不要直接操作切片破坏顺序。
- 调试日志在 `~/.config/nga-tui/debug.log`；任何涉及登录/session 的改动都应加 `debug.Logf`（只打印 cookie 名，不打印值）。

### 登录

终端内无法完成密码登录（需验证码）。两条路径：按 `B` 用 CDP 启动独立 Chrome 轮询 cookie；按 `M` 手动粘贴 `ngaPassportUid=...; ngaPassportCid=...`。启动时 `CheckLogin` 网络失败会乐观保留登录态，只有明确 2048 才清标记。

## 测试约定

- 测试全部用标准库 + `httptest`，不依赖真实网络；不要给 CI 加外网测试。
- `internal/api` 和 `internal/ui` 各有 `TestMain`，把 `HOME` 指向临时目录，避免覆盖真实配置和 debug.log。新增会读写 `~/.config` 的包时需照做。
- 新增 NGA JSON 变体时，在 `internal/api/api_test.go` 里用真实响应片段补 `fixJSON`/解析用例。
