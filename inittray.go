package main

import "context"

// initTray 启动托盘（纯 D-Bus 实现，无系统依赖）
func initTray(ctx context.Context, onShow, onQuit func()) {
	startTray(ctx, onShow, onQuit)
}
