package bbcode

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 常用内联样式。
var (
	bold          = lipgloss.NewStyle().Bold(true)
	underline     = lipgloss.NewStyle().Underline(true)
	italic        = lipgloss.NewStyle().Italic(true)
	strikethrough = lipgloss.NewStyle().Strikethrough(true)
	dim           = lipgloss.NewStyle().Faint(true)

	accentColor = lipgloss.Color("#ff8c00")
)

// colorMap 把 NGA 常用颜色名映射到 lipgloss 颜色。
var colorMap = map[string]string{
	"red":       "#ff5555",
	"blue":      "#55aaff",
	"green":     "#55ff55",
	"darkblue":  "#3366aa",
	"gray":      "#8a8a8a",
	"grey":      "#8a8a8a",
	"silver":    "#c0c0c0",
	"orange":    "#ffa500",
	"brown":     "#a0522d",
	"darkred":   "#aa3333",
	"purple":    "#aa55ff",
	"pink":      "#ff88aa",
	"skyblue":   "#87ceeb",
	"white":     "#ffffff",
	"black":     "#000000",
	"yellow":    "#ffff55",
	"olive":     "#808000",
	"teal":      "#008080",
	"navy":      "#000080",
	"lime":      "#00ff00",
	"maroon":    "#800000",
	"aqua":      "#00ffff",
	"fuchsia":   "#ff00ff",
	"gold":      "#ffd700",
	"platinum":  "#e5e4e2",
	"skyblue2":  "#7ec0ee",
	"crimson":   "#dc143c",
	"coral":     "#ff7f50",
	"seagreen":  "#2e8b57",
	"indigo":    "#4b0082",
	"tomato":    "#ff6347",
	"salmon":    "#fa8072",
	"chocolate": "#d2691e",
}

// colorStyle 根据颜色名（或直接颜色值）构造样式。
func colorStyle(attr string) lipgloss.Style {
	attr = strings.TrimSpace(attr)
	if attr == "" {
		return lipgloss.NewStyle()
	}
	if hex, ok := colorMap[strings.ToLower(attr)]; ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
	}
	// 支持 #rrggbb 形式
	if strings.HasPrefix(attr, "#") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(attr))
	}
	return lipgloss.NewStyle().Foreground(accentColor)
}
