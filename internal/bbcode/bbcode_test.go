package bbcode

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func displayWidth(s string) int { return ansi.StringWidthWc(s) }

func TestRenderPlain(t *testing.T) {
	out := Render("你好，世界 [s:ac:黑枪]", 40)
	if strings.Contains(out, "[s:") {
		t.Fatalf("表情标签未剔除: %q", out)
	}
	if !strings.Contains(out, "你好") {
		t.Fatalf("文本丢失: %q", out)
	}
}

func TestRenderBoldAndColor(t *testing.T) {
	out := Render("[b]加粗[/b] [color=red]红色[/color]", 40)
	// 内容保留、标签被剥掉（ANSI 码依赖终端色彩支持，测试环境不输出）
	if !strings.Contains(out, "加粗") || strings.Contains(out, "[b]") {
		t.Fatalf("加粗内容异常: %q", out)
	}
	if !strings.Contains(out, "红色") || strings.Contains(out, "[color") {
		t.Fatalf("颜色内容异常: %q", out)
	}
}

func TestRenderQuote(t *testing.T) {
	out := Render("前面[quote]引用内容[/quote]后面", 40)
	if !strings.Contains(out, "│") {
		t.Fatalf("引用竖线缺失: %q", out)
	}
}

func TestRenderURL(t *testing.T) {
	out := Render("[url=https://example.com]点这里[/url]", 60)
	if !strings.Contains(out, "点这里") || !strings.Contains(out, "example.com") {
		t.Fatalf("URL 渲染异常: %q", out)
	}
}

func TestRenderImg(t *testing.T) {
	out := Render("[img]https://example.com/a.jpg[/img]", 40)
	if strings.Contains(out, "example.com") {
		t.Fatalf("图片不应显示 URL: %q", out)
	}
	if !strings.Contains(out, "图片") {
		t.Fatalf("图片占位缺失: %q", out)
	}
}

func TestRenderBr(t *testing.T) {
	out := Render("第一行<br/>第二行", 40)
	if !strings.Contains(out, "\n") {
		t.Fatalf("<br/> 未转成换行: %q", out)
	}
}

func TestWrapLongLine(t *testing.T) {
	long := strings.Repeat("这是很长的一段中文内容用于测试换行 ", 10)
	out := Render(long, 30)
	maxW := 0
	for _, line := range strings.Split(out, "\n") {
		w := displayWidth(line)
		if w > maxW {
			maxW = w
		}
	}
	if maxW > 30 {
		t.Fatalf("折行后最大宽度 %d 超过 30", maxW)
	}
}

func TestQuoteWrapKeepsBar(t *testing.T) {
	content := "[quote]" + strings.Repeat("引用的长内容 ", 20) + "[/quote]"
	out := Render(content, 30)
	lines := strings.Split(out, "\n")
	hasBar := 0
	for _, ln := range lines {
		if strings.HasPrefix(ln, "│ ") {
			hasBar++
		}
	}
	if hasBar < 2 {
		t.Fatalf("长引用应折行且续行带竖线，实际带竖线行数=%d: %q", hasBar, out)
	}
}

func TestRenderURLAsVisibleText(t *testing.T) {
	cases := []string{
		"[url]https://v.douyin.com/9B-Rm96-GKI/[/url]",
		"看这个 https://v.douyin.com/9B-Rm96-GKI/",
	}
	for _, in := range cases {
		out := Render(in, 200)
		if !strings.Contains(out, "https://v.douyin.com/9B-Rm96-GKI/") {
			t.Fatalf("URL 被截断: 输入 %q，输出 %q", in, out)
		}
	}
}

func TestRenderSmileStillStripped(t *testing.T) {
	out := Render("笑[s:ac:哭笑]好 s:ac:goodjob 结束", 80)
	if strings.Contains(out, "s:") || strings.Contains(out, "[]") {
		t.Fatalf("表情标记未剔除干净: %q", out)
	}
	if !strings.Contains(out, "笑") || !strings.Contains(out, "好") || !strings.Contains(out, "结束") {
		t.Fatalf("表情周边文本丢失: %q", out)
	}
}

func TestQuoteKeepsBarAndWidth(t *testing.T) {
	// 强制彩色输出，否则 lipgloss 不产生 ANSI 前缀，测不到续行竖线问题
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })

	long := strings.Repeat("这是一段很长的引用内容", 8)
	src := "[quote][b]Post by 某人[/b]<br/><br/>" + long + "[/quote]<br/><br/>层主回复的具体内容"
	out := Render(src, 40)
	lines := strings.Split(out, "\n")

	lastBar := -1
	for i, ln := range lines {
		if strings.Contains(ln, "│") {
			lastBar = i
		}
		if w := ansi.StringWidthWc(ln); w > 40 {
			t.Fatalf("第 %d 行宽 %d 超过 40（引用块不应被补齐撑宽）: %q", i, w, ln)
		}
	}
	if lastBar < 0 {
		t.Fatal("引用块没有竖线")
	}
	// 引用块内的非空行都必须带竖线
	for i := 0; i <= lastBar; i++ {
		plain := strings.TrimSpace(ansi.Strip(lines[i]))
		if plain == "" {
			continue
		}
		if !strings.Contains(lines[i], "│") {
			t.Fatalf("引用续行丢了竖线: %q", lines[i])
		}
	}
	// 引用之后的回复不应被当成引用
	for i := lastBar + 1; i < len(lines); i++ {
		if strings.Contains(lines[i], "│") {
			t.Fatalf("回复内容不应带引用竖线: %q", lines[i])
		}
	}
	if !strings.Contains(out, "层主回复") {
		t.Fatalf("回复内容丢失: %q", out)
	}
}
