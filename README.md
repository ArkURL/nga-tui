<p align="center">
  <a href="https://github.com/ArkURL/nga-tui"><img src="https://img.shields.io/github/stars/ArkURL/nga-tui?style=social" alt="GitHub Stars"></a>
  <a href="https://go.dev/dl/"><img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.22+"></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img src="https://img.shields.io/badge/Built%20with-Bubble%20Tea-ff69b4" alt="Built with Bubble Tea"></a>
  <a href="https://github.com/ArkURL/nga-tui"><img src="https://img.shields.io/github/languages/top/ArkURL/nga-tui" alt="Top Language"></a>
  <a href="https://github.com/ArkURL/nga-tui/blob/main/LICENSE"><img src="https://img.shields.io/github/license/ArkURL/nga-tui" alt="License"></a>
</p>

# nga-tui

NGA 论坛终端客户端（TUI），基于 Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea) 构建。

刷帖、看回复、版面导航与搜索、本地收藏；登录通过浏览器自动抓取 Cookie，无需在终端输入密码。

## 功能

- **刷贴**：浏览版面的帖子列表，支持翻页、按最后回复/发布时间排序、刷新
- **看回复**：进入帖子查看主楼 + 回复（BBCode 渲染，支持引用块、颜色、加粗、链接、图片占位）
- **切分论坛**：按"分类 → 分组 → 版面"三级导航，支持收藏常用版面（`f` 收藏/取消、`Tab` 切换「全部/收藏」），或直接搜索版面（搜索结果里也可 `f` 收藏）。已有收藏时，启动首页直接进入收藏版面
- **子版面切换**：进入版面默认只看帖子；若有子版面，按 `t` 切换到单独的子版面列表（合集带 `[合集]` 标记），回车钻取，`t` 切回帖子视图，子版面也可 `f` 收藏
- **直达版面**：版面视图按 `o`，粘贴 NGA 链接（如 `bbs.nga.cn/thread.php?stid=47206901`）或填 `fid`/`stid` 直接打开任意版面（含分类树外的深层子版面）
- **搜索**：当前版面内搜帖；按名称搜索版面（本地匹配已加载分类）
- **登录**：可选。匿名可浏览；登录后可访问需要登录的版面（如网事杂谈）。支持手动粘贴 Cookie 兜底

## 安装

```bash
go install github.com/ArkURL/nga-tui@latest
# 或本地运行
go run .
```

需要 Go 1.22+。

## 使用

```bash
nga-tui
```

启动后进入版面列表，按 `?` 查看全部键位。

### 键位

| 按键 | 功能 |
|------|------|
| `j` / `k` / `↑` / `↓` | 移动光标；阅读页内按楼跳转 |
| `Shift+J` / `Shift+K`（阅读页） | 逐行细调滚动 |
| `Enter` / `l` | 进入当前项 |
| `Esc` / `h` / `q` | 返回上级（根级 `q` 退出） |
| `n` / `p` / `PgDn` / `PgUp` | 翻页 |
| `g` / `G` | 跳到顶部 / 底部 |
| `/` | 搜索（版面视图搜版面，帖子视图版内搜帖） |
| `f`（版面/搜索结果） | 收藏 / 取消收藏版面 |
| `Tab`（版面视图） | 切换「全部 / 收藏」版面 |
| `e` | 切换排序（最后回复 / 发布时间） |
| `r` | 刷新当前列表 |
| `L` | 登录 / 登出 |
| `?` | 帮助 |
| `Ctrl+C` | 强制退出 |

### 登录

按 `L` 进入登录视图。NGA 网页登录需要验证码，终端内无法完成密码登录，统一使用浏览器登录：

1. **浏览器自动抓取（按 `B`，推荐）**：自动打开一个独立的 Chrome 窗口进入 bbs.nga.cn，
   你在窗口里登录（可正常处理验证码），登录完成后自动抓取 `ngaPassportUid` / `ngaPassportCid` 并保存到本地
2. **手动粘贴 Cookie（按 `M`，兜底）**：在浏览器登录 bbs.nga.cn 后，从开发者工具复制
   `ngaPassportUid=...; ngaPassportCid=...` 粘贴即可

Cookie 保存在 `~/.config/nga-tui/config.json`（0600 权限），启动时自动恢复并验证会话。

## 配置

| 路径 | 说明 |
|------|------|
| `~/.config/nga-tui/config.json` | 登录 Cookie 等（自动维护，无需手动编辑） |

## 目录结构

```
.
├── main.go                  # 入口
├── internal/
│   ├── api/                 # NGA 数据层（HTTP 客户端、JSON 修复、分类/帖子/内容/登录接口）
│   ├── bbcode/              # BBCode → ANSI 渲染
│   ├── config/              # 配置读写
│   ├── model/               # 数据模型
│   └── ui/                  # bubbletea 视图（app 路由 + 各视图）
└── cmd/verify/              # 开发辅助：验证 NGA API
```

## 说明

- 基于 NGA 网页版 JSON 接口（`__output=11`），浏览接口无需登录
- 请求使用移动端 UA `NGA_WP_JW`，并对响应做宽松 JSON 解析（数字 key、控制字符等兼容）
- 登录通过浏览器自动抓取 Cookie（CDP 驱动独立 Chrome 实例）或手动粘贴，无需终端内输密码
- 图片不下载，显示 `[图片]` 占位

## 开发

```bash
go test ./...    # 单元测试
go run ./cmd/verify        # 验证数据层接口
go run ./cmd/verify <tid>  # 渲染指定帖子内容
```

## 发布（CI/CD）

- **CI**：推送/PR 到 `main` 自动跑 `go vet` + `go test`
- **发布**：打 `v*` 标签后，GitHub Actions 用 GoReleaser 交叉编译（linux/darwin/windows × amd64/arm64）并发布到 GitHub Releases

```bash
git tag v0.1.0
git push origin v0.1.0
```

发布产物含各平台压缩包与 `checksums.txt` 校验文件，可在 [Releases](https://github.com/ArkURL/nga-tui/releases) 页面下载。
