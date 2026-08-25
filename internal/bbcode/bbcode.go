// Package bbcode 把 NGA 帖子内容（BBCode）渲染为终端可显示的 ANSI 文本。
package bbcode

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Node 是 BBCode 解析树节点。
type Node struct {
	Tag     string  // 小写标签名；文本节点为空
	Attr    string  // [tag=attr] 的参数
	Text    []*Node // 标签节点的子节点
	Content string  // 文本节点内容
}

// Parse 把 BBCode 源码解析为树。
func Parse(src string) *Node {
	root := &Node{}
	var stack []*Node
	cur := root

	i := 0
	for i < len(src) {
		if src[i] == '[' {
			if close := matchCloseTag(src, i); close != "" {
				if cur.Tag == close {
					// 正常闭合：回到父节点
					if len(stack) > 0 {
						cur = stack[len(stack)-1]
						stack = stack[:len(stack)-1]
					}
				} else if parent := popUntil(&stack, close); parent != nil {
					// 嵌套不匹配：弹栈到匹配标签
					cur = parent
				}
				i += len(close) + 3 // [/ + name + ]
				continue
			}
			if open, attr, end := matchOpenTag(src, i); open != "" {
				child := &Node{Tag: open, Attr: attr}
				cur.Text = append(cur.Text, child)
				stack = append(stack, cur)
				cur = child
				i = end
				continue
			}
		}
		next := strings.IndexByte(src[i:], '[')
		if next < 0 {
			if run := src[i:]; run != "" {
				cur.Text = append(cur.Text, &Node{Content: run})
			}
			break
		}
		if run := src[i : i+next]; run != "" {
			cur.Text = append(cur.Text, &Node{Content: run})
		}
		i += next
	}
	return root
}

func matchOpenTag(src string, i int) (tag, attr string, end int) {
	endBracket := strings.IndexByte(src[i:], ']')
	if endBracket < 0 {
		return "", "", -1
	}
	endBracket += i
	inner := strings.TrimSpace(src[i+1 : endBracket])
	if inner == "" || strings.HasPrefix(inner, "/") {
		return "", "", -1
	}
	if eq := strings.IndexByte(inner, '='); eq >= 0 {
		return strings.ToLower(strings.TrimSpace(inner[:eq])), strings.TrimSpace(inner[eq+1:]), endBracket + 1
	}
	return strings.ToLower(inner), "", endBracket + 1
}

func matchCloseTag(src string, i int) string {
	if i+2 >= len(src) || src[i+1] != '/' {
		return ""
	}
	endBracket := strings.IndexByte(src[i:], ']')
	if endBracket < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(src[i+2 : i+endBracket]))
}

// popUntil 从栈顶向下找匹配标签，找不到返回 nil。
func popUntil(stack *[]*Node, tag string) *Node {
	for len(*stack) > 0 {
		parent := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		if parent.Tag == tag {
			return parent
		}
	}
	return nil
}

var smileRe = regexp.MustCompile(`s:[^\s\[\]]+`)

// quoteBar 是引用块的左侧竖线。
const quoteBar = "│ "

// Render 把 BBCode 渲染为已按 width 折行的 ANSI 文本。
func Render(src string, width int) string {
	if width <= 0 {
		width = 80
	}
	root := Parse(src)
	out := renderNode(root, width)
	return wrapFinal(out, width)
}

func renderNode(n *Node, width int) string {
	inner := renderInner(n, width)
	switch n.Tag {
	case "":
		return inner
	case "quote":
		return renderQuote(inner, width)
	case "b", "strong":
		return bold.Render(inner)
	case "u":
		return underline.Render(inner)
	case "i", "em":
		return italic.Render(inner)
	case "del", "strike":
		return strikethrough.Render(inner)
	case "color":
		return colorStyle(n.Attr).Render(inner)
	case "url":
		return renderURL(n, inner)
	case "img":
		return dim.Render("[图片]")
	case "collapse", "hide":
		return dim.Render("[折叠内容]")
	case "list":
		return renderList(n, width)
	default:
		// 未知标签：剥掉标签，保留内容
		return inner
	}
}

// renderInner 渲染节点的自身文本与所有子节点（按源码顺序）。
func renderInner(n *Node, width int) string {
	var sb strings.Builder
	if n.Content != "" {
		sb.WriteString(escapeText(n.Content))
	}
	for _, ch := range n.Text {
		sb.WriteString(renderNode(ch, width))
	}
	return sb.String()
}

func renderQuote(inner string, width int) string {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return ""
	}
	lines := strings.Split(inner, "\n")
	for i := range lines {
		lines[i] = quoteBar + lines[i]
	}
	return "\n" + dim.Render(strings.Join(lines, "\n")) + "\n"
}

func renderURL(n *Node, inner string) string {
	link := n.Attr
	text := strings.TrimSpace(inner)
	if text == "" {
		text = link
	}
	if link == "" {
		return underline.Render(text)
	}
	return underline.Render(text) + " " + dim.Render("("+link+")")
}

func renderList(n *Node, width int) string {
	var sb strings.Builder
	for _, ch := range n.Text {
		if ch.Tag == "*" {
			sb.WriteString("• " + renderInner(ch, width))
			sb.WriteString("\n")
		} else {
			sb.WriteString(renderNode(ch, width))
		}
	}
	return sb.String()
}

// escapeText 清理文本：剔除表情标签、转换 <br/>。
func escapeText(s string) string {
	s = smileRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// wrapFinal 对最终输出的每一行做宽度折行；对带引用竖线的行，
// 把前缀保留在续行上。
func wrapFinal(out string, width int) string {
	lines := strings.Split(out, "\n")
	var res []string
	for _, ln := range lines {
		if ansi.StringWidthWc(ln) <= width {
			res = append(res, ln)
			continue
		}
		res = append(res, wrapLine(ln, width)...)
	}
	return strings.Join(res, "\n")
}

func wrapLine(ln string, width int) []string {
	// 提取行首引用竖线前缀
	var prefix strings.Builder
	rest := ln
	for strings.HasPrefix(rest, quoteBar) {
		prefix.WriteString(quoteBar)
		rest = rest[len(quoteBar):]
	}
	prefixStr := prefix.String()
	if prefixStr == "" {
		return strings.Split(ansi.HardwrapWc(ln, width, true), "\n")
	}
	innerWidth := max(width-ansi.StringWidthWc(prefixStr), 10)
	wrapped := ansi.HardwrapWc(rest, innerWidth, true)
	lines := strings.Split(wrapped, "\n")
	for i := range lines {
		lines[i] = prefixStr + lines[i]
	}
	return lines
}
