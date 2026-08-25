package bbcode

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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
