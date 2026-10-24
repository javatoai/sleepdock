package main

import (
	"image/png"
	"log"
	"os"

	"github.com/egoist/mygo/ui"
)

func main() {
	if err := os.MkdirAll("assets", 0755); err != nil {
		log.Fatal(err)
	}
	svg := ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20.5 13a8.8 8.8 0 0 1-9.5-9.5A9 9 0 1 0 20.5 13Z"/><path d="M17 3v4m-2-2h4"/></svg>`))
	for _, name := range []string{"icon", "tray", "tray-color"} {
		size := 64
		if name == "icon" {
			size = 1024
		}
		im := ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
			box := ui.Box(c).Fill().Center()
			color := ui.Hex("#000000")
			iconSize := float32(48)
			if name == "icon" {
				box.Radius(220).Background(ui.Hex("#6757D9"))
				color = ui.Hex("#FFFFFF")
				iconSize = 670
			} else if name == "tray-color" {
				color = ui.Hex("#9585EE")
			}
			box.Children(func() { ui.Icon(c, svg).Size(iconSize, iconSize).TextColor(color) })
		}, size, size, 1)
		f, err := os.Create("assets/" + name + ".png")
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, im); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}
}
