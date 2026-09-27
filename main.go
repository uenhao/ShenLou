package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 单实例握手：同版本唤醒旧窗口；版本更新时旧实例退出让位
	switch handshake() {
	case "show", "show-legacy":
		println("蜃楼 TV 已在运行，已请求显示原窗口")
		return
	}
	app := NewApp()
	listenShowRequests(app)

	err := wails.Run(&options.App{
		Title:     "蜃楼 TV",
		Width:     1280,
		Height:    820,
		MinWidth:  1000,
		MinHeight: 680,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
		Linux: &linux.Options{
			ProgramName: "ShenLou",
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
