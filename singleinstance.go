package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// buildID 由 build.sh 通过 -ldflags 注入（构建时间戳），
// 用于“新版替换旧版”握手：版本不同时旧实例自动退出让位。
var buildID = "dev"

// 单实例唤醒协议（unix socket）：
//
//	新实例 → 已运行实例："version <buildID>"
//	同版本 → 回复 "same"，再发 "show" 唤醒窗口
//	不同版本 → 回复 "stale"，再发 "quit"，旧实例退出，新实例正常启动
func showSocketPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "ShenLou-show.sock")
}

// handshake 返回 "show"（同版本已唤醒）/ "show-legacy"（旧协议实例已唤醒）/ "none"（无实例）
func handshake() string {
	conn, err := net.DialTimeout("unix", showSocketPath(), time.Second)
	if err != nil {
		return "none"
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1500 * time.Millisecond))
	if _, err := fmt.Fprintf(conn, "version %s\n", buildID); err != nil {
		return "show-legacy"
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	switch strings.TrimSpace(line) {
	case "same":
		_, _ = fmt.Fprintln(conn, "show")
		return "show"
	case "stale":
		_, _ = fmt.Fprintln(conn, "quit")
		time.Sleep(800 * time.Millisecond) // 等旧实例退出
		return "replaced"
	}
	_ = err
	return "show-legacy"
}

// listenShowRequests 监听唤醒/替换请求
func listenShowRequests(app *App) {
	path := showSocketPath()
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveInstanceConn(conn, app)
		}
	}()
}

func serveInstanceConn(conn net.Conn, app *App) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return
	}
	switch fields[0] {
	case "version":
		same := len(fields) > 1 && fields[1] == buildID
		if same {
			_, _ = fmt.Fprintln(conn, "same")
		} else {
			_, _ = fmt.Fprintln(conn, "stale")
		}
		cmd, err2 := r.ReadString('\n')
		cmd = strings.TrimSpace(cmd)
		if err2 != nil && cmd == "" {
			return
		}
		switch cmd {
		case "show":
			app.showSelf()
		case "quit":
			if !same {
				app.selfQuit() // 只有版本不同才允许被替换
			}
		}
	case "show":
		app.showSelf()
	}
}
