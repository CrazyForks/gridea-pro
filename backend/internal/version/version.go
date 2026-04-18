// Package version 暴露 Gridea Pro 的产品名与版本号。
//
// 重要：Version 常量必须与 wails.json 的 info.productVersion 保持一致，
// 发版时一并更新。
package version

const (
	// Product 是产品名称，用于 <meta name="generator"> 等场景。
	Product = "Gridea Pro"

	// Version 是当前版本号。
	Version = "1.0.0"
)

// Generator 返回 <meta name="generator"> 的 content 值。
func Generator() string {
	return Product + " " + Version
}
