package main

import (
	"embed"
	"gridea-pro/backend/pkg/boot"
)

// 构建时由 CI 通过 -ldflags 注入，本地开发默认显示 "dev"
var Version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	boot.Run(assets, Version)
}
