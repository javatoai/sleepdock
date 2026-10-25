package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sleepdock/internal/desktop"

	"github.com/egoist/mygo/ui"
)

func main() {
	out := flag.String("out", "artifacts", "离线界面预览目录")
	flag.Parse()
	if err := render(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func render(out string) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	for _, state := range []string{"idle", "at", "running", "grace", "completed", "error"} {
		v := desktop.Preview(state)
		tt := ui.NewTester(v.Render, 560, 590)
		f, err := os.Create(filepath.Join(out, state+".png"))
		if err != nil {
			return err
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	fmt.Println("已生成六种离线界面预览")
	return nil
}
