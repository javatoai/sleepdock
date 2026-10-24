package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sleepdock/internal/desktop"
	"sleepdock/internal/storage"
)

func main() {
	dir := flag.String("config-dir", "", "使用独立设置目录")
	version := flag.Bool("version", false, "显示版本")
	flag.Parse()
	if *version {
		fmt.Println("SleepDock 0.1.0")
		return
	}
	path, err := storage.DefaultPath()
	if *dir != "" {
		path, err = filepath.Abs(filepath.Join(*dir, "settings.json"))
	}
	if err == nil {
		err = desktop.Run(path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
