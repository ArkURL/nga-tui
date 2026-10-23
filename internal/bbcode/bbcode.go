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
				// NGA 表情 [s:xxx] 是自闭合的，没有 [/s:xxx]。
				// 当成普通文本交给 escapeText 清理，避免它一直挂在标签栈上，
				// 导致后面的 [/quote] 错位、把层主回复也吞进引用块。
				if strings.HasPrefix(open, "s:") {
					cur.Text = append(cur.Text, &Node{Content: src[i:end]})
					i = end
					continue
				}
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

// popUntil 处理错位闭合：从栈顶向下找到被闭合的标签，
// 返回它的父节点（即闭合后 cur 应该回到的位置），找不到返回 nil。
// 注意不能返回被闭合的标签本身，否则闭合标签之后的内容会继续挂在它下面。
func popUntil(stack *[]*Node, tag string) *Node {
	for len(*stack) > 0 {
		n := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		if n.Tag == tag {
			if len(*stack) == 0 {
				return nil
			}
			parent := (*stack)[len(*stack)-1]
			*stack = (*stack)[:len(*stack)-1]
			return parent
		}
	}
	return nil
}

var (
	// smileTagRe 匹配带方括号的表情标记，如 [s:ac:哭笑]。
	smileTagRe = regexp.MustCompile(`\[s:[^\]]*\]`)
	// smileRawRe 匹配裸表情标记，如 s:ac:哭笑。
	// 前面的字符不能是 ASCII 字母/数字，否则会把 https:// 里的 "s://..."
	// 当成表情删掉，链接只剩 "http"。
	smileRawRe = regexp.MustCompile(`(^|[^\w])s:[^\s\[\]]+`)
)

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
	// 逐行渲染：对整段多行字符串调用 lipgloss 会把每行补齐到最长行宽，
	// 导致引用块被撑宽后再被 wrapFinal 二次折行，续行还会丢掉竖线。
	lines := strings.Split(inner, "\n")
	for i := range lines {
		lines[i] = dim.Render(quoteBar + lines[i])
	}
	return "\n" + strings.Join(lines, "\n") + "\n"
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

// smileLabel 把 NGA 表情代码转成文字描述，如 "s:ac:哭笑" -> "哭笑"。
func smileLabel(code string) string {
	code = strings.TrimPrefix(code, "s:")
	if i := strings.LastIndexByte(code, ':'); i >= 0 {
		code = code[i+1:]
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "表情"
	}
	return code
}

// renderSmile 把表情渲染为文字占位，如 〔哭笑〕。
func renderSmile(code string) string {
	return "〔" + smileLabel(code) + "〕"
}

// escapeText 处理文本：表情转为文字占位、转换 <br/>。
func escapeText(s string) string {
	s = smileTagRe.ReplaceAllStringFunc(s, func(tag string) string {
		code := strings.TrimSuffix(strings.TrimPrefix(tag, "[s:"), "]")
		return renderSmile(code)
	})
	s = smileRawRe.ReplaceAllStringFunc(s, func(tok string) string {
		// 匹配里带了表情前的一个字符（避免误伤 https:），原样保留
		if i := strings.Index(tok, "s:"); i >= 0 {
			return tok[:i] + renderSmile(tok[i:])
		}
		return tok
	})
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

// takeQuotePrefix 拆出行首的 ANSI 样式和引用竖线，
// 返回可直接拼在续行前的前缀（保留原有的 dim 样式）。
func takeQuotePrefix(ln string) (prefix, rest string) {
	i := 0
	for i < len(ln) {
		if ln[i] == 0x1b {
			i = ansiSeqEnd(ln, i)
			continue
		}
		if strings.HasPrefix(ln[i:], quoteBar) {
			i += len(quoteBar)
			return ln[:i], ln[i:]
		}
		break
	}
	return "", ln
}

// ansiSeqEnd 返回从 s[i]（ESC）开始的转义序列结束位置。
func ansiSeqEnd(s string, i int) int {
	j := i + 1
	if j >= len(s) {
		return j
	}
	if s[j] != '[' {
		return j + 1
	}
	j++
	for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
		j++
	}
	if j < len(s) {
		j++
	}
	return j
}

func wrapLine(ln string, width int) []string {
	prefix, rest := takeQuotePrefix(ln)
	if prefix == "" {
		return strings.Split(ansi.HardwrapWc(ln, width, true), "\n")
	}
	innerWidth := max(width-ansi.StringWidthWc(prefix), 10)
	wrapped := ansi.HardwrapWc(rest, innerWidth, true)
	lines := strings.Split(wrapped, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return lines
}
