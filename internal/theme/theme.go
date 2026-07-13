package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Accent     lipgloss.Color
	Border     lipgloss.Color
	Muted      lipgloss.Color
}

var Order = []string{
	"Nord",
	"Dracula",
	"Gruvbox",
	"Tokyo Night",
	"Solarized Dark",
	"Solarized Light",
}

var themes = map[string]Theme{
	"Nord":            {Name: "Nord", Background: "#2E3440", Foreground: "#D8DEE9", Accent: "#88C0D0", Border: "#4C566A", Muted: "#4C566A"},
	"Dracula":         {Name: "Dracula", Background: "#282A36", Foreground: "#F8F8F2", Accent: "#BD93F9", Border: "#44475A", Muted: "#6272A4"},
	"Gruvbox":         {Name: "Gruvbox", Background: "#282828", Foreground: "#EBDBB2", Accent: "#FE8019", Border: "#504945", Muted: "#928374"},
	"Tokyo Night":     {Name: "Tokyo Night", Background: "#1A1B26", Foreground: "#C0CAF5", Accent: "#7AA2F7", Border: "#3B4261", Muted: "#565F89"},
	"Solarized Dark":  {Name: "Solarized Dark", Background: "#002B36", Foreground: "#839496", Accent: "#268BD2", Border: "#073642", Muted: "#586E75"},
	"Solarized Light": {Name: "Solarized Light", Background: "#FDF6E3", Foreground: "#657B83", Accent: "#268BD2", Border: "#EEE8D5", Muted: "#93A1A1"},
}

func Get(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return themes["Nord"]
}

func Next(name string) string {
	for i, n := range Order {
		if n == name {
			return Order[(i+1)%len(Order)]
		}
	}
	return Order[0]
}
