package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Station 播放列表中的一个频道（沿用 m3u 的 name/url 结构）
type Station struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// App 应用主体，导出方法由 Wails 绑定给前端调用。
// 视频播放在前端 <video> 元素内完成，后端只负责数据、录制与窗口/托盘。
type App struct {
	ctx      context.Context
	mu       sync.Mutex
	quitting bool           // 真退出标志：让 beforeClose 放行
	history  []HistoryEntry // 观看历史（最近优先，去重，上限 historyMax）
}

func NewApp() *App { return &App{} }

// setCtx/getCtx：ctx 的写入（startup）与读取（单实例回调 showSelf 可能先于
// startup 完成）存在并发，统一纳入 a.mu 避免 data race
func (a *App) setCtx(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
}

func (a *App) getCtx() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx
}

func (a *App) startup(ctx context.Context) {
	a.setCtx(ctx)
	// 终端关闭（SIGHUP）/ kill（SIGTERM、SIGINT）也走优雅退出：
	// 否则 OnShutdown 不执行，正在录制的 vlc 会变成无人管理的孤儿进程
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	go func() {
		first := true
		for range sigCh {
			if first {
				first = false
				a.forceQuit()
			} else {
				os.Exit(1) // 优雅退出卡住时，第二次信号直接硬退
			}
		}
	}()
	ensureFavoriteList()
	ensureBuiltinPlaylists()
	a.loadHistory()
	initTray(ctx, a.showSelf, func() { a.forceQuit() })
	// libVLC 内嵌引擎（失败不致命：<video> 引擎照常，VLC 兜底退为外部窗口）
	if err := initVlcEmbed(); err != nil {
		fmt.Fprintln(os.Stderr, "[vlc-embed]", err)
		// 启动瞬间主窗标题可能尚未就绪：后台重试直到成功
		go func() {
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				if err := initVlcEmbed(); err == nil {
					return
				}
			}
		}()
	}
}

func (a *App) shutdown(ctx context.Context) {
	stopAllRecordings()
	a.shutdownVlcEmbed()
}

// beforeClose 关闭按钮/Alt+F4 → 隐藏到后台（播放、录制继续），返回 true 阻止真正退出。
// 真正退出（托盘“退出”/新版本替换）会先置 quitting，此时放行。
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	quitting := a.quitting
	a.mu.Unlock()
	if quitting {
		return false
	}
	runtime.WindowHide(ctx)
	return true
}

// forceQuit 真正退出应用（绕过“关闭即隐藏”）。清理由 OnShutdown 完成。
func (a *App) forceQuit() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	if ctx := a.getCtx(); ctx != nil {
		runtime.Quit(ctx)
	}
}

// showSelf 唤醒窗口（单实例协议回调，可能早于 startup 完成）
func (a *App) showSelf() {
	if ctx := a.getCtx(); ctx != nil {
		runtime.WindowShow(ctx)
		runtime.WindowUnminimise(ctx)
	}
}

// selfQuit 被新版本替换时自动退出
func (a *App) selfQuit() { a.forceQuit() }

// ---------- 无边框窗口控制 ----------

// MinimiseWindow 最小化
func (a *App) MinimiseWindow() { runtime.WindowMinimise(a.ctx) }

// ToggleMaximiseWindow 最大化 / 还原
func (a *App) ToggleMaximiseWindow() { runtime.WindowToggleMaximise(a.ctx) }

// HideWindow 隐藏到托盘（后台保持播放）
func (a *App) HideWindow() { runtime.WindowHide(a.ctx) }

// FullscreenWindowOn 窗口全屏（VLC 内嵌引擎的全屏：整窗全屏 + 前端舞台铺满）
func (a *App) FullscreenWindowOn() { runtime.WindowFullscreen(a.ctx) }

// FullscreenWindowOff 退出窗口全屏
func (a *App) FullscreenWindowOff() { runtime.WindowUnfullscreen(a.ctx) }

func vlcBin() string {
	for _, p := range []string{"vlc", "cvlc"} {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return ""
}

// VlcAvailable 前端启动时检查 VLC 是否可用（录制与"在 VLC 中打开"依赖它）
func (a *App) VlcAvailable() bool { return vlcBin() != "" }

// OpenExternalVlc 用带界面的 VLC 窗口打开一个流地址/文件。
// 内嵌 <video> 放不了的协议（rtsp 等）与录像回看的兜底通道。
func (a *App) OpenExternalVlc(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("地址为空")
	}
	bin := vlcBin()
	if bin == "" {
		return fmt.Errorf("未找到 vlc，请先安装：sudo apt install vlc")
	}
	return exec.Command(bin, url).Start() // 用户主动开的 VLC 窗口允许独立存活
}

// ---------- 观看历史（history.json，最近优先，按地址去重） ----------

// HistoryEntry 一条观看历史
type HistoryEntry struct {
	Name string    `json:"name"`
	URL  string    `json:"url"`
	At   time.Time `json:"at"`
}

const historyMax = 100

func historyPath() (string, error) {
	root, err := dataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "history.json"), nil
}

func (a *App) loadHistory() {
	p, err := historyPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var h []HistoryEntry
	if json.Unmarshal(data, &h) != nil {
		return
	}
	a.mu.Lock()
	a.history = h
	a.mu.Unlock()
}

func (a *App) saveHistory() {
	a.mu.Lock()
	h := append([]HistoryEntry(nil), a.history...)
	a.mu.Unlock()
	p, err := historyPath()
	if err != nil {
		return
	}
	data, err := json.Marshal(h)
	if err != nil {
		return
	}
	tmp := p + ".tmp"
	if os.WriteFile(tmp, data, 0644) == nil {
		_ = os.Rename(tmp, p)
	}
}

// recordHistory 记一次观看：同一地址去重并置顶
func (a *App) recordHistory(name, url string) {
	if strings.HasPrefix(url, "/") {
		return
	}
	a.mu.Lock()
	out := make([]HistoryEntry, 0, len(a.history)+1)
	out = append(out, HistoryEntry{Name: name, URL: url, At: time.Now()})
	for _, e := range a.history {
		if e.URL != url {
			out = append(out, e)
		}
	}
	if len(out) > historyMax {
		out = out[:historyMax]
	}
	a.history = out
	a.mu.Unlock()
	a.saveHistory()
}

// RecordHistory 前端在频道成功开播时调用（<video> 的 playing 事件）
func (a *App) RecordHistory(name, url string) { a.recordHistory(name, url) }

// PlayHistory 返回观看历史（最近优先）
func (a *App) PlayHistory() []HistoryEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]HistoryEntry(nil), a.history...)
}

// ClearHistory 清空观看历史
func (a *App) ClearHistory() error {
	a.mu.Lock()
	a.history = nil
	a.mu.Unlock()
	p, err := historyPath()
	if err != nil {
		return err
	}
	_ = os.Remove(p)
	return nil
}

// ReportError 前端把未捕获的 JS 错误回报到后端日志，便于排查
func (a *App) ReportError(msg string) {
	fmt.Fprintln(os.Stderr, "[JS-ERROR]", msg)
}
