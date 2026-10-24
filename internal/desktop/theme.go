package desktop

import (
	"math"

	"github.com/egoist/mygo/ui"
)

type palette struct{ bg, surface, soft, line, text, muted, accent, tint, danger, success ui.Color }

func theme(c *ui.Context, preference string) palette {
	p := palette{ui.Hex("#F6F5F9"), ui.Hex("#FFFFFF"), ui.Hex("#F3F2F8"), ui.Hex("#E7E5F0"), ui.Hex("#25263A"), ui.Hex("#787B91"), ui.Hex("#6757D9"), ui.Hex("#EEEBFC"), ui.Hex("#C95666"), ui.Hex("#369476")}
	t := ui.LightTheme()
	if preference == "dark" || (preference == "system" && c.Theme().Dark) {
		p = palette{ui.Hex("#11121C"), ui.Hex("#1A1C2A"), ui.Hex("#232536"), ui.Hex("#303248"), ui.Hex("#EFF0FA"), ui.Hex("#A1A3BB"), ui.Hex("#A99BFF"), ui.Hex("#302A51"), ui.Hex("#F59DA9"), ui.Hex("#8BD8BC")}
		t = ui.DarkTheme()
	}
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = p.bg, p.surface, p.soft, p.tint
	t.Text, t.TextMuted, t.Border = p.text, p.muted, p.line
	t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#6757D9"), ui.Hex("#5948C8"), ui.Hex("#4E3FB5")
	t.AccentText, t.Danger = ui.Hex("#FFFFFF"), p.danger
	t.Radius, t.FontSize, t.Spacing = 12, 14, 4
	c.SetTheme(t)
	return p
}

var paths = map[string]string{
	"moon":     `<path d="M20.5 13a8.8 8.8 0 0 1-9.5-9.5A9 9 0 1 0 20.5 13Z"/>`,
	"star":     `<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5z"/>`,
	"clock":    `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
	"play":     `<path d="m8 5 11 7-11 7z"/>`,
	"close":    `<path d="m6 6 12 12M18 6 6 18"/>`,
	"plus":     `<path d="M12 5v14M5 12h14"/>`,
	"check":    `<path d="m5 12 4 4L19 6"/>`,
	"settings": `<path d="m9 3-1 3-3 1-2 5 2 5 3 1 1 3h6l1-3 3-1 2-5-2-5-3-1-1-3z"/><circle cx="12" cy="12" r="3"/>`,
	"warning":  `<circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 3v1"/>`,
	"tray":     `<path d="M3 5h18v14H3zM8 22h8m-4-3v3M7 9h10M9 13h6"/>`,
	"reset":    `<path d="M4 8a8 8 0 1 1 0 8M4 3v5h5"/>`,
}
var icons = func() map[string]*ui.SVG {
	m := map[string]*ui.SVG{}
	for name, path := range paths {
		m[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return m
}()

func icon(c *ui.Context, name string, size float32, color ui.Color) {
	ui.Icon(c, icons[name]).Size(size, size).TextColor(color).Shrink(0)
}
func iconButton(c *ui.Context, key, name, label string, p palette) bool {
	b := ui.ButtonBase(c.Key(key)).Size(34, 34).Radius(10).Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(p.soft)
	}
	b.Children(func() { icon(c, name, 18, p.muted) })
	return b.Clicked()
}
func button(c *ui.Context, key, name, label string, p palette, primary, disabled bool) bool {
	b := ui.ButtonBase(c.Key(key)).Grow(1).Height(46).Padding(0, 23).Row().Gap(9).Center().Radius(13).Label(label).Disabled(disabled)
	fg, bg := p.text, p.soft
	if primary {
		fg, bg = ui.Hex("#FFFFFF"), ui.Hex("#6757D9")
		if b.Hovered() {
			bg = ui.Hex("#5948C8")
		}
	} else if b.Hovered() {
		bg = p.tint
	}
	if disabled {
		fg, bg = p.muted, p.soft
	}
	b.Background(bg).Children(func() { icon(c, name, 17, fg); ui.Text(c, label).FontSize(14).Bold().TextColor(fg).NoWrap() })
	return b.Clicked()
}

func ring(c *ui.Context, p palette, progress float64, warning bool, children func()) {
	color := p.accent
	if warning {
		color = p.danger
	}
	ui.Box(c).Size(286, 286).AlignSelf(ui.Center).Center().Draw(func(painter *ui.Painter, r ui.Rect) {
		cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2-5
		circle := func(part float64) *ui.Path {
			path := &ui.Path{}
			steps := max(1, int(part*144))
			for i := 0; i <= steps; i++ {
				a := float64(i)/float64(steps)*part*2*math.Pi - math.Pi/2
				x, y := cx+rad*float32(math.Cos(a)), cy+rad*float32(math.Sin(a))
				if i == 0 {
					path.MoveTo(x, y)
				} else {
					path.LineTo(x, y)
				}
			}
			return path
		}
		painter.StrokePath(circle(1), 3, p.line)
		if progress > 0 {
			painter.StrokePath(circle(min(1, progress)), 4, color)
		}
	}).Children(children)
}
