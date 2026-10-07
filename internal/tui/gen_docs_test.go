//go:build docs

package tui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/profile"
)

var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// TestGenerateDocs renders the picker with sample data into docs/picker.svg.
// Run: go test -tags docs -run TestGenerateDocs ./internal/tui
func TestGenerateDocs(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)

	store := &profile.Store{Profiles: []profile.Profile{
		{Name: "personal", GitName: "Ada Lovelace", Email: "ada@gmail.com", GitHub: "ada-codes"},
		{Name: "work", GitName: "Ada Lovelace", Email: "ada@acme.dev", GitHub: "ada-acme"},
	}}
	m := model{
		store:  store,
		cursor: 1,
		cwd:    `C:\code\acme\api`,
		global: gitx.Identity{Name: "Ada Lovelace", Email: "ada@acme.dev"},
		here:   gitx.Identity{Name: "Ada Lovelace", Email: "ada@acme.dev"},
		links:  []gitx.Link{{Dir: "C:/code/acme/", File: filepath.ToSlash(store.ConfigFile(store.Profiles[1]))}},
		msg:    "Switched globally to work",
	}
	svg := toSVG(m.View())
	out := filepath.Join("..", "..", "docs", "picker.svg")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func toSVG(ansi string) string {
	const (
		cw, lh, pad = 8.4, 20.0, 24.0
		defFg       = "#e5e7eb"
	)
	lines := strings.Split(strings.TrimRight(ansi, "\n"), "\n")
	cols := 0
	var body strings.Builder
	for i, line := range lines {
		fg, bold := defFg, false
		var row strings.Builder
		pos, width := 0, 0
		for _, loc := range sgr.FindAllStringSubmatchIndex(line, -1) {
			text := line[pos:loc[0]]
			if text != "" {
				row.WriteString(span(text, fg, bold))
				width += utf8.RuneCountInString(text)
			}
			fg, bold = applySGR(line[loc[2]:loc[3]], fg, bold, defFg)
			pos = loc[1]
		}
		if rest := line[pos:]; rest != "" {
			row.WriteString(span(rest, fg, bold))
			width += utf8.RuneCountInString(rest)
		}
		if width > cols {
			cols = width
		}
		fmt.Fprintf(&body, `<text x="%.0f" y="%.0f" xml:space="preserve">%s</text>`+"\n", pad, pad+lh*float64(i+1)-5, row.String())
	}
	w := pad*2 + cw*float64(cols)
	h := pad*2 + lh*float64(len(lines))
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img" aria-label="gitswitch interactive picker showing a personal and a work profile">
<rect width="100%%" height="100%%" rx="10" fill="#111827"/>
<g font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,'DejaVu Sans Mono',monospace" font-size="14" fill="%s">
%s</g>
</svg>
`, w, h, w, h, defFg, body.String())
}

func span(text, fg string, bold bool) string {
	weight := ""
	if bold {
		weight = ` font-weight="bold"`
	}
	return fmt.Sprintf(`<tspan fill="%s"%s>%s</tspan>`, fg, weight, html.EscapeString(text))
}

func applySGR(params, fg string, bold bool, def string) (string, bool) {
	if params == "" {
		return def, false
	}
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case "0":
			fg, bold = def, false
		case "1":
			bold = true
		case "38":
			if i+4 < len(p) && p[i+1] == "2" {
				r, _ := strconv.Atoi(p[i+2])
				g, _ := strconv.Atoi(p[i+3])
				b, _ := strconv.Atoi(p[i+4])
				fg = fmt.Sprintf("#%02x%02x%02x", r, g, b)
				i += 4
			}
		}
	}
	return fg, bold
}
