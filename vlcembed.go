package main

// libVLC 内嵌引擎：在 Wails 顶层窗口内创建一个 X11 子窗口，
// libVLC 把视频直接画进去；用于 <video> 播不了的源（rtsp、野服务器、怪时间戳）。
// 坐标由前端按视频舞台的布局实时上报（SetVideoRect），子窗口随主窗移动自动跟随。

/*
#cgo pkg-config: libvlc
#cgo LDFLAGS: -lX11
#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>

static Display *xd_open() { return XOpenDisplay(""); }

static void xd_close(Display *d) { XCloseDisplay(d); }

// 判定"真正的应用主窗"：WM_CLASS 含 ShenLou 且 _NET_WM_NAME 含 "蜃楼"(UTF-8)。
// GTK/Wails 会遗留一个同名 10x10 的未映射幽灵窗口，靠标题唯一区分。
static int is_shenlou_main(Display *d, Window w, Atom cls, Atom netName) {
	unsigned char *cp = NULL; unsigned long ci, cl; int cf; Atom ct;
	int ok = 0;
	if (XGetWindowProperty(d, w, cls, 0, 128, False, AnyPropertyType,
		&ct, &cf, &ci, &cl, &cp) == Success && cp) {
		if (strstr((char *)cp, "ShenLou")) {
			unsigned char *np = NULL; unsigned long ni, nl; int nf; Atom nt;
			if (XGetWindowProperty(d, w, netName, 0, 256, False, AnyPropertyType,
				&nt, &nf, &ni, &nl, &np) == Success && np) {
				if (strstr((char *)np, "\xe8\x9c\x83\xe6\xa5\xbc")) ok = 1; // 蜃楼
				XFree(np);
			}
		}
		XFree(cp);
	}
	return ok;
}

static Window find_toplevel(Display *d) {
	Atom cls = XInternAtom(d, "WM_CLASS", True);
	Atom netName = XInternAtom(d, "_NET_WM_NAME", True);
	if (cls == None || netName == None) return 0;

	// 路径一：EWMH 客户窗口列表
	Atom netCl = XInternAtom(d, "_NET_CLIENT_LIST", True);
	if (netCl != None) {
		Atom type; int fmt; unsigned long items, left; unsigned char *prop = NULL;
		if (XGetWindowProperty(d, DefaultRootWindow(d), netCl, 0, 1024, False, AnyPropertyType,
			&type, &fmt, &items, &left, &prop) == Success && prop) {
			Window *wins = (Window *)prop;
			for (unsigned long i = 0; i < items; i++) {
				if (is_shenlou_main(d, wins[i], cls, netName)) {
					Window f = wins[i];
					XFree(prop);
					return f;
				}
			}
			XFree(prop);
		}
	}

	// 路径二：根窗口直接孩子兜底（同样过标题判据）
	Window root = DefaultRootWindow(d);
	Window r, p2, *ch; unsigned int n;
	if (!XQueryTree(d, root, &r, &p2, &ch, &n)) return 0;
	Window found = 0;
	for (unsigned int i = 0; i < n; i++) {
		if (is_shenlou_main(d, ch[i], cls, netName)) { found = ch[i]; break; }
	}
	XFree(ch);
	return found;
}

static Window create_child(Display *d, Window parent) {
	Window w = XCreateSimpleWindow(d, parent, 0, 0, 480, 270, 0,
		BlackPixel(d, DefaultScreen(d)), BlackPixel(d, DefaultScreen(d)));
	XSelectInput(d, w, StructureNotifyMask | ExposureMask);
	// 不在此处 Map：由 VlcPlay 成功后再显示，避免启动期出现黑块
	XSync(d, False);
	return w;
}

static void move_resize(Display *d, Window w, int x, int y, unsigned int ww, unsigned int hh) {
	XMoveResizeWindow(d, w, x, y, ww, hh);
	XRaiseWindow(d, w);
	XSync(d, False);
}

static void move_offscreen(Display *d, Window w) {
	// 隐藏=移到屏幕外而非 Unmap：libVLC 的 xlib 视频输出遇到未映射窗口会失效，
	// 之后即使重新 Map 也无法恢复出画面（音频不受影响）
	XMoveWindow(d, w, -10000, -10000);
	XSync(d, False);
}

static void show_hide(Display *d, Window w, int show) {
	if (show) XMapRaised(d, w); else XUnmapWindow(d, w);
	XSync(d, False);
}
*/
import "C"

import (
	"fmt"
	"os"
	"sync"

	vlc "github.com/adrg/libvlc-go/v3"
)

type vlcEmbed struct {
	mu        sync.Mutex
	disp      *C.Display
	win       C.Window
	player    *vlc.Player
	media     *vlc.Media
	state     string // idle | opening | playing | paused | error
	running   bool
	vlcInited bool // vlc.Init 只允许成功一次；窗口未就绪的重试不得重复初始化
	quitting  bool // shutdown 已开始：重试路径不得再复活引擎
}

var vemb vlcEmbed

// initVlcEmbed 启动时初始化 libVLC 与 X11 子窗口（失败则整个引擎不可用，前端只走 <video>）
func initVlcEmbed() error {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if vemb.running {
		return nil
	}
	if vemb.quitting {
		// 已在退出流程：别让 startup 的重试 goroutine 在 shutdown 之后"复活"引擎，
		// 否则会留下无人清理的 X 子窗口/Display
		return fmt.Errorf("应用正在退出")
	}
	// 启动窗口期 find_toplevel 可能失败，随后会反复重试进这里；
	// vlc.Init 必须只做一次（重复初始化会泄漏 libvlc 实例）
	if !vemb.vlcInited {
		if err := vlc.Init("--no-osd", "--avcodec-hw=any"); err != nil {
			return fmt.Errorf("libvlc 初始化失败：%w", err)
		}
		vemb.vlcInited = true
	}
	d := C.xd_open()
	if d == nil {
		return fmt.Errorf("X11 显示连接失败")
	}
	parent := C.find_toplevel(d)
	if parent == 0 {
		C.xd_close(d)
		return fmt.Errorf("未找到 ShenLou 顶层窗口")
	}
	vemb.disp = d
	vemb.win = C.create_child(d, parent)
	fmt.Fprintf(os.Stderr, "[vlc-embed] 就绪：toplevel=0x%x child=0x%x\n", uint64(parent), uint64(vemb.win))
	vemb.state = "idle"
	vemb.running = true
	return nil
}

func (a *App) shutdownVlcEmbed() {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	vemb.quitting = true
	if !vemb.running {
		return
	}
	vemb.detachLocked()
	d, w := vemb.disp, vemb.win
	go func() { C.show_hide(d, w, 0); C.xd_close(d) }()
	vemb.running = false
}

// detachLocked 摘下当前播放器（不持有任何锁地善后：libVLC 的 Stop/Release
// 在个别状态下会阻塞，绝不能在锁内调用，否则轮询/退出全会排队卡死）
func (e *vlcEmbed) detachLocked() {
	pl, m := e.player, e.media
	e.player, e.media = nil, nil
	e.state = "idle"
	if pl != nil {
		go func() {
			pl.Stop()
			if m != nil {
				m.Release()
			}
			pl.Release()
		}()
	}
}

// VlcEmbeddedAvailable 前端启动时探测内嵌引擎是否可用
func (a *App) VlcEmbeddedAvailable() bool {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	return vemb.running
}

// VlcPlay 用内嵌引擎播放一个地址
func (a *App) VlcPlay(url string) error {
	// 启动时窗口可能尚未就绪导致初始化失败；首次播放再试一次
	if !func() bool { vemb.mu.Lock(); defer vemb.mu.Unlock(); return vemb.running }() {
		if err := initVlcEmbed(); err != nil {
			fmt.Fprintln(os.Stderr, "[vlc-embed] 播放期初始化失败:", err)
			return fmt.Errorf("内嵌引擎不可用：%w", err)
		}
	}
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	fmt.Fprintln(os.Stderr, "[vlc-embed] 播放:", url)
	vemb.detachLocked()
	player, err := vlc.NewPlayer()
	if err != nil {
		vemb.state = "error"
		return err
	}
	if err := player.SetXWindow(uint32(vemb.win)); err != nil {
		fmt.Fprintln(os.Stderr, "[vlc-embed] SetXWindow 失败:", err)
		go player.Release() // Release 可能阻塞，不能在锁内同步调（此时 player 未挂到 vemb，锁外 goroutine 安全）
		vemb.state = "error"
		return err
	}
	media, err := player.LoadMediaFromURL(url)
	if err != nil {
		go player.Release()
		vemb.state = "error"
		return err
	}
	C.show_hide(vemb.disp, vemb.win, 1)
	vemb.player = player
	vemb.media = media
	vemb.state = "opening"
	if err := player.Play(); err != nil {
		vemb.state = "error"
		return err
	}
	return nil
}

// VlcStop 停止并隐藏内嵌画面
func (a *App) VlcStop() {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if !vemb.running {
		return
	}
	vemb.detachLocked()
	C.show_hide(vemb.disp, vemb.win, 0)
}

// VlcState 前端轮询：idle | opening | playing | paused | error
func (a *App) VlcState() string {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if vemb.running && vemb.player != nil {
		// libvlc 事件线程无回调时，用播放器状态兜底刷新
		if vemb.state != "error" {
			if vemb.player.IsPlaying() {
				if vemb.state != "paused" {
					vemb.state = "playing"
				}
			} else if vemb.state == "playing" || vemb.state == "opening" {
				// 失去 playing（含 Opening/Buffering 期，libvlc 这些状态 IsPlaying 为假）：
				// 死流常在 Opening 阶段就转 Error，直播断流多落在 Ended/Error/Stopped。
				// 置为 error 让前端走换流/断流重连（paused 态由显式暂停写入，不在此处理）
				if st, err := vemb.player.MediaState(); err == nil {
					switch st {
					case vlc.MediaEnded, vlc.MediaError, vlc.MediaStopped:
						vemb.state = "error"
					}
				}
			}
		}
	}
	return vemb.state
}

// VlcSetPaused 暂停/恢复
func (a *App) VlcSetPaused(p bool) {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if vemb.player == nil {
		return
	}
	vemb.player.SetPause(p)
	// 两个方向都要显式回写：VlcState 的兜底刷新对 paused 态有豁免
	// （防止 libvlc 在暂停边缘误报 IsPlaying 把状态冲掉），恢复时若不
	// 写回 playing，轮询会永远返回 paused，前端把 UI 打回暂停态
	if p {
		vemb.state = "paused"
	} else {
		vemb.state = "playing"
	}
}

// VlcSetVolume 0-100
func (a *App) VlcSetVolume(pct int) error {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if vemb.player == nil {
		return nil
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return vemb.player.SetVolume(pct)
}

// VlcSetMute 静音开关
func (a *App) VlcSetMute(m bool) {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if vemb.player != nil {
		vemb.player.SetMute(m)
	}
}

// SetVideoRect 前端上报视频舞台在窗口内的坐标（随布局变化调用）
func (a *App) SetVideoRect(x, y, w, h int) {
	vemb.mu.Lock()
	defer vemb.mu.Unlock()
	if !vemb.running || vemb.win == 0 {
		return
	}
	if w < 2 || h < 2 {
		C.move_offscreen(vemb.disp, vemb.win)
		return
	}
	C.move_resize(vemb.disp, vemb.win, C.int(x), C.int(y), C.uint(w), C.uint(h))
}
