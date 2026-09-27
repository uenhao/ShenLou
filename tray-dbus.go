package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

//go:embed icon.png
var trayIconPNG []byte

//go:embed icon.svg
var trayIconSVG []byte

// ensureThemeIcon 把矢量图标安装到用户图标目录（无需 root），
// 供任务栏/窗口按 WM_CLASS（ShenLou / shenlou）查找窗口图标。
func ensureThemeIcon() (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	dir := filepath.Join(home, ".local", "share", "icons", "hicolor", "scalable", "apps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", ""
	}
	_ = os.WriteFile(filepath.Join(dir, "ShenLou.svg"), trayIconSVG, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "shenlou.svg"), trayIconSVG, 0o644)
	return "ShenLou", filepath.Join(home, ".local", "share", "icons")
}

// 纯 Go 的 D-Bus StatusNotifierItem 托盘实现：
// 不依赖 libayatana-appindicator（无需 sudo 安装任何系统库），
// 直接与桌面环境的 StatusNotifierWatcher（xapp / KDE / GNOME 扩展）通信。
//
// 左键托盘 → 显示窗口；右键菜单 → 显示窗口 / 退出。

const (
	sniNameBase = "org.kde.StatusNotifierItem"
	sniPath     = "/StatusNotifierItem"
	menuPath    = "/MenuBar"
	sniIface    = "org.kde.StatusNotifierItem"
	menuIface   = "com.canonical.dbusmenu"
	watcherDest = "org.kde.StatusNotifierWatcher"
	watcherPath = "/StatusNotifierWatcher"
	menuIdShow  = 1
	menuIdQuit  = 3
)

type dbusMenuItem struct {
	Id         int32
	Properties map[string]dbus.Variant
}

type menuLayout struct {
	Id         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

// sniPixmap 对应 SNI 的 (iiay)：宽、高、ARGB32 大端像素
type sniPixmap struct {
	Width  int32
	Height int32
	Data   []byte
}

// sniTooltip 对应 SNI 的 (sa(iiay)ss)
type sniTooltip struct {
	IconName    string
	IconPixmaps []sniPixmap
	Title       string
	Description string
}

// sniObject 实现 org.kde.StatusNotifierItem 的方法
type sniObject struct{ onActivate func() }

func (o *sniObject) Activate(x int32, y int32) *dbus.Error {
	if o.onActivate != nil {
		go o.onActivate()
	}
	return nil
}

func (o *sniObject) SecondaryActivate(x int32, y int32) *dbus.Error {
	if o.onActivate != nil {
		go o.onActivate()
	}
	return nil
}

func (o *sniObject) Scroll(delta int32, orientation string) *dbus.Error { return nil }

// menuObject 实现 com.canonical.dbusmenu
type menuObject struct {
	onShow func()
	onQuit func()
	rev    uint32
}

func (m *menuObject) GetLayout(parentId int32, recursionDepth int32, propertyList []string) (uint32, menuLayout, *dbus.Error) {
	m.rev++
	root := menuLayout{
		Id:         0,
		Properties: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")},
		Children: []dbus.Variant{
			dbus.MakeVariant(menuLayout{Id: menuIdShow, Properties: map[string]dbus.Variant{
				"label": dbus.MakeVariant("显示窗口"), "enabled": dbus.MakeVariant(true)}}),
			dbus.MakeVariant(menuLayout{Id: 2, Properties: map[string]dbus.Variant{
				"type": dbus.MakeVariant("separator")}}),
			dbus.MakeVariant(menuLayout{Id: menuIdQuit, Properties: map[string]dbus.Variant{
				"label": dbus.MakeVariant("退出 蜃楼"), "enabled": dbus.MakeVariant(true)}}),
		},
	}
	return m.rev, root, nil
}

// menuItemsFlat 返回全部菜单项属性（GetGroupProperties 与 GetLayout 共用）
func menuItemsFlat() []dbusMenuItem {
	return []dbusMenuItem{
		{Id: menuIdShow, Properties: map[string]dbus.Variant{
			"label": dbus.MakeVariant("显示窗口"), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true)}},
		{Id: 2, Properties: map[string]dbus.Variant{
			"type": dbus.MakeVariant("separator"), "visible": dbus.MakeVariant(true)}},
		{Id: menuIdQuit, Properties: map[string]dbus.Variant{
			"label": dbus.MakeVariant("退出 蜃楼"), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true)}},
	}
}

func (m *menuObject) GetGroupProperties(ids []int32, names []string) ([]dbusMenuItem, *dbus.Error) {
	m.rev++
	all := menuItemsFlat()
	if len(ids) == 0 {
		return all, nil
	}
	want := map[int32]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := []dbusMenuItem{}
	for _, it := range all {
		if want[it.Id] {
			out = append(out, it)
		}
	}
	return out, nil
}

func (m *menuObject) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	for _, it := range menuItemsFlat() {
		if it.Id != id {
			continue
		}
		if v, ok := it.Properties[name]; ok {
			return v, nil
		}
	}
	return dbus.MakeVariant(false), nil
}

func (m *menuObject) Event(id int32, eventId string, data dbus.Variant, timestamp int32) *dbus.Error {
	if eventId == "clicked" {
		switch id {
		case menuIdShow:
			if m.onShow != nil {
				go m.onShow()
			}
		case menuIdQuit:
			if m.onQuit != nil {
				go m.onQuit()
			}
		}
	}
	return nil
}

func (m *menuObject) AboutToShow(id int32) (bool, *dbus.Error) { return true, nil }

// iconToARGB 把嵌入的 PNG 转为 SNI 的 ARGB32 大端像素（非预乘）
func iconToARGB() (int32, int32, []byte, error) {
	img, err := png.Decode(bytes.NewReader(trayIconPNG))
	if err != nil {
		return 0, 0, nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, 0, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA() // 16 位预乘
			var px uint32
			if a16 > 0 {
				r8 := uint32((r16*255 + a16/2) / a16) // 反预乘到 8 位
				g8 := uint32((g16*255 + a16/2) / a16)
				b8 := uint32((b16*255 + a16/2) / a16)
				a8 := uint32(a16 >> 8)
				px = a8<<24 | r8<<16 | g8<<8 | b8
			}
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], px)
			out = append(out, buf[:]...)
		}
	}
	return int32(w), int32(h), out, nil
}

// startTray 启动托盘（纯 D-Bus，无 CGO 依赖）
func startTray(ctx context.Context, onShow, onQuit func()) {
	go func() {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			fmt.Fprintln(os.Stderr, "[tray] 连接会话总线失败:", err)
			return
		}
		iconName, themePath := ensureThemeIcon()
		name := fmt.Sprintf("%s-%d-1", sniNameBase, os.Getpid())
		if _, err := conn.RequestName(name, dbus.NameFlagDoNotQueue); err != nil {
			fmt.Fprintln(os.Stderr, "[tray] 注册总线名失败:", err)
			return
		}

		sni := &sniObject{onActivate: onShow}
		if err := conn.Export(sni, sniPath, sniIface); err != nil {
			fmt.Fprintln(os.Stderr, "[tray] 导出 SNI 失败:", err)
			return
		}

		pixValue := []sniPixmap{}
		tooltipValue := sniTooltip{IconName: iconName, Title: "蜃楼 TV", Description: "蜃楼正在后台观看"}
		if w, h, data, err := iconToARGB(); err == nil {
			pixValue = []sniPixmap{{Width: w, Height: h, Data: data}}
			tooltipValue.IconPixmaps = pixValue
		}

		props := map[string]map[string]*prop.Prop{
			sniIface: {
				"Category":        {Value: "ApplicationStatus", Writable: false, Emit: prop.EmitTrue},
				"Id":              {Value: "ShenLou", Writable: false, Emit: prop.EmitTrue},
				"Title":           {Value: "蜃楼 TV", Writable: false, Emit: prop.EmitTrue},
				"Status":          {Value: "Active", Writable: false, Emit: prop.EmitTrue},
				"IconThemePath":   {Value: themePath, Writable: false, Emit: prop.EmitTrue},
				"IconName":        {Value: "", Writable: false, Emit: prop.EmitTrue},
				"IconPixmap":      {Value: pixValue, Writable: false, Emit: prop.EmitTrue},
				"AttentionPixmap": {Value: pixValue, Writable: false, Emit: prop.EmitTrue},
				"Menu":            {Value: dbus.ObjectPath(menuPath), Writable: false, Emit: prop.EmitTrue},
				"ToolTip":         {Value: tooltipValue, Writable: false, Emit: prop.EmitTrue},
				"ItemIsMenu":      {Value: false, Writable: false, Emit: prop.EmitTrue},
			},
		}
		if _, err := prop.Export(conn, sniPath, props); err != nil {
			fmt.Fprintln(os.Stderr, "[tray] 导出属性失败:", err)
		}

		menu := &menuObject{onShow: onShow, onQuit: onQuit}
		if err := conn.Export(menu, menuPath, menuIface); err != nil {
			fmt.Fprintln(os.Stderr, "[tray] 导出菜单失败:", err)
		}
		menuProps := map[string]map[string]*prop.Prop{
			menuIface: {
				"Version": {Value: uint32(2), Writable: false, Emit: prop.EmitTrue},
			},
		}
		if _, err := prop.Export(conn, menuPath, menuProps); err != nil {
			fmt.Fprintln(os.Stderr, "[tray] 导出菜单属性失败:", err)
		}

		// 向 watcher 注册；托盘服务可能晚于应用启动，失败则重试
		obj := conn.Object(watcherDest, watcherPath)
		for i := 0; i < 30; i++ {
			call := obj.Call(watcherDest+".RegisterStatusNotifierItem", 0, name)
			if call.Err == nil {
				fmt.Fprintln(os.Stderr, "[tray] 托盘图标已注册:", name)
				// 发送就绪信号：宿主（如 xapp）依赖这些信号拉取图标/标题/菜单，
				// 不发的话只会显示占位圆圈
				for _, sig := range []string{"NewIcon", "NewTitle", "NewToolTip", "NewMenu"} {
					_ = conn.Emit(sniPath, sniIface+"."+sig)
				}
				_ = conn.Emit(sniPath, sniIface+".NewStatus", "Active")
				return
			}
			time.Sleep(2 * time.Second)
		}
		fmt.Fprintln(os.Stderr, "[tray] 注册托盘失败（未检测到 StatusNotifierWatcher）")
	}()
}
