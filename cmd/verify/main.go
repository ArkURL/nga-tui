// 开发辅助工具：验证 NGA API 各接口。
//
// 用法:
//
//	verify            # 验证分类 + 帖子列表 + 搜索 + 登录状态
//	verify <tid>      # 渲染指定帖子的前几楼（含 BBCode）
package main

import (
	"fmt"
	"os"

	"github.com/ArkURL/nga-tui/internal/api"
	"github.com/ArkURL/nga-tui/internal/bbcode"
)

func main() {
	c := api.NewClient()
	if len(os.Args) >= 2 {
		renderThread(c, os.Args[1])
		return
	}

	cats, err := c.GetCategories()
	if err != nil {
		fmt.Println("分类 ERROR:", err)
	} else {
		fmt.Println("分类数:", len(cats))
	}

	res, err := c.GetThreads("7", 1, "lastpostdesc", "")
	if err != nil {
		fmt.Println("列表 ERROR:", err)
	} else {
		fmt.Printf("列表 page=%d pages=%d threads=%d\n", res.Page, res.Pages, len(res.Threads))
		for i, th := range res.Threads {
			if i >= 3 {
				break
			}
			fmt.Printf("  tid=%d replies=%d %s | %s\n", th.TID, th.Replies, th.Subject, th.Author)
		}
	}

	sres, err := c.GetThreads("7", 1, "lastpostdesc", "魔兽")
	if err != nil {
		fmt.Println("搜索 ERROR:", err)
	} else {
		fmt.Printf("搜索 page=%d pages=%d threads=%d\n", sres.Page, sres.Pages, len(sres.Threads))
	}

	ok, err := c.CheckLogin()
	fmt.Printf("登录状态: ok=%v err=%v\n", ok, err)
}

func renderThread(c *api.Client, tidStr string) {
	var tid int
	fmt.Sscanf(tidStr, "%d", &tid)
	content, err := c.GetThreadContent(tid, 1)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("page=%d pages=%d rows=%d replies=%d users=%d\n",
		content.Page, content.Pages, content.Rows, len(content.Replies), len(content.Users))
	for i, r := range content.Replies {
		if i >= 3 {
			break
		}
		u := content.Users[r.AuthorID]
		fmt.Printf("\n=== #%d %s (%s) ===\n", r.Lou, u.Username, r.PostDate)
		fmt.Println(bbcode.Render(r.Content, 80))
	}
}
