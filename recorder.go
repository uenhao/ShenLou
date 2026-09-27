package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// RecordingInfo 录制条目（列表/回听/导出用）
type RecordingInfo struct {
	ID        string `json:"id"`        // 文件名（含扩展名），唯一标识
	Name      string `json:"name"`      // 电台名
	URL       string `json:"url"`       // 流地址（仅录制中的条目有值）
	File      string `json:"file"`      // 完整路径
	Size      int64  `json:"size"`      // 字节数
	StartedAt string `json:"startedAt"` // 开始时间 2006-01-02 15:04:05
	Duration  string `json:"duration"`  // 时长 mm:ss 或 h:mm:ss
	Active    bool   `json:"active"`    // 是否正在录制
}

type recordingProc struct {
	cmd   *exec.Cmd
	url   string
	start time.Time
}

var (
	recMu     sync.Mutex
	recActive = map[string]*recordingProc{}
)

// recNameRe 匹配录制文件名：<电台名>_<YYYYMMDD-HHMMSS>[_序号]
var recNameRe = regexp.MustCompile(`^(.*)_(\d{8}-\d{6})(?:_\d+)?$`)

func recordingsDir() (string, error) {
	root, err := dataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "recordings"), nil
}

// RecordingsDir 前端展示录制目录位置用
func (a *App) RecordingsDir() string {
	dir, err := recordingsDir()
	if err != nil {
		return ""
	}
	return dir
}

// sanitizeFilename 电台名转安全文件名片段
func sanitizeFilename(name string) string {
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "-", "*", "", "?", "", "\"", "'",
		"<", "(", ">", ")", "|", "-", "\n", " ", "\r", "",
	)
	n := strings.TrimSpace(r.Replace(name))
	if n == "" {
		n = "电台"
	}
	if rn := []rune(n); len(rn) > 60 {
		n = string(rn[:60])
	}
	return n
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// StartRecording 录制指定频道（前端把当前正在观看的频道传进来）。
// 单独起一个无界面 vlc 拉同一路流写入 TS 文件，因此同一频道可以有多个并行录制，
// 录制不影响正常观看。
func (a *App) StartRecording(st Station) (RecordingInfo, error) {
	st.Name = strings.TrimSpace(st.Name)
	st.URL = strings.TrimSpace(st.URL)
	if st.URL == "" {
		return RecordingInfo{}, errors.New("频道地址为空")
	}
	if st.Name == "" {
		st.Name = st.URL
	}
	bin := vlcBin()
	if bin == "" {
		return RecordingInfo{}, errors.New("未找到 vlc，请先安装：sudo apt install vlc")
	}
	dir, err := recordingsDir()
	if err != nil {
		return RecordingInfo{}, err
	}
	start := time.Now()
	recMu.Lock()
	base := fmt.Sprintf("%s_%s", sanitizeFilename(st.Name), start.Format("20060102-150405"))
	id := base + ".ts"
	path := filepath.Join(dir, id)
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("%s_%d.ts", base, i)
		path = filepath.Join(dir, id)
	}
	// 命名/查重/注册整体在锁内：同一秒双击"开始录制"不会双双通过查重写进同一文件
	cmd := exec.Command(bin, "-I", "dummy", st.URL,
		"--sout=#standard{access=file,mux=ts,dst="+path+"}")
	// 主进程意外退出时由内核终止录制 vlc，避免孤儿进程持续写盘
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	if err := cmd.Start(); err != nil {
		recMu.Unlock()
		return RecordingInfo{}, fmt.Errorf("启动录制失败：%w", err)
	}
	recActive[id] = &recordingProc{cmd: cmd, url: st.URL, start: start}
	recMu.Unlock()
	go func() {
		_ = cmd.Wait()
		recMu.Lock()
		delete(recActive, id)
		recMu.Unlock()
	}()
	return RecordingInfo{
		ID: id, Name: st.Name, URL: st.URL, File: path,
		StartedAt: start.Format("2006-01-02 15:04:05"),
		Duration:  "00:00", Active: true,
	}, nil
}

// StopRecording 停止指定录制（TS 流按包写入，中断处的文件仍可正常回听）
func (a *App) StopRecording(id string) error {
	recMu.Lock()
	rp := recActive[id]
	recMu.Unlock()
	if rp == nil {
		return fmt.Errorf("录制不存在或已结束")
	}
	if rp.cmd != nil && rp.cmd.Process != nil {
		_ = rp.cmd.Process.Kill()
	}
	return nil
}

func activeRecordingIDs() []string {
	recMu.Lock()
	defer recMu.Unlock()
	ids := make([]string, 0, len(recActive))
	for id := range recActive {
		ids = append(ids, id)
	}
	return ids
}

func stopAllRecordings() {
	for _, id := range activeRecordingIDs() {
		recMu.Lock()
		rp := recActive[id]
		recMu.Unlock()
		if rp != nil && rp.cmd != nil && rp.cmd.Process != nil {
			_ = rp.cmd.Process.Kill()
		}
	}
}

// ListRecordings 列出全部录制文件（录制中的排在按时间倒序的自然位置，带 Active 标记）。
// vlc 启动初期 .ts 可能尚未落盘——已注册的活跃录制即使没有文件也要出现在列表里，
// 否则前端"活跃录制消失"对比会把它误判为异常结束。
func (a *App) ListRecordings() ([]RecordingInfo, error) {
	dir, err := recordingsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	list := []RecordingInfo{}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		onDisk[e.Name()] = true
		list = append(list, buildRecInfo(dir, e.Name(), fi))
	}
	recMu.Lock()
	for id, rp := range recActive {
		if onDisk[id] {
			continue
		}
		info := RecordingInfo{ID: id, URL: rp.url, Active: true,
			File: filepath.Join(dir, id), StartedAt: rp.start.Format("2006-01-02 15:04:05")}
		if m := recNameRe.FindStringSubmatch(strings.TrimSuffix(id, filepath.Ext(id))); m != nil {
			info.Name = m[1]
		} else {
			info.Name = id
		}
		list = append(list, info)
	}
	recMu.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].StartedAt > list[j].StartedAt })
	return list, nil
}

func buildRecInfo(dir, fname string, fi os.FileInfo) RecordingInfo {
	id := fname
	base := strings.TrimSuffix(fname, filepath.Ext(fname))
	name := base
	var startT time.Time
	if m := recNameRe.FindStringSubmatch(base); m != nil {
		name = m[1]
		if t, err := time.ParseInLocation("20060102-150405", m[2], time.Local); err == nil {
			startT = t
		}
	}
	recMu.Lock()
	rp := recActive[id]
	recMu.Unlock()
	active := rp != nil
	endT := fi.ModTime()
	if active {
		startT = rp.start
		endT = time.Now()
	} else if startT.IsZero() {
		startT = fi.ModTime()
	}
	url := ""
	if active {
		url = rp.url
	}
	return RecordingInfo{
		ID: id, Name: name, URL: url, File: filepath.Join(dir, fname),
		Size: fi.Size(), StartedAt: startT.Format("2006-01-02 15:04:05"),
		Duration: fmtDur(endT.Sub(startT)), Active: active,
	}
}

// ExportRecording 把录制文件复制到 ~/Downloads（重名自动追加时间后缀）
func (a *App) ExportRecording(id string) (string, error) {
	dir, err := recordingsDir()
	if err != nil {
		return "", err
	}
	clean := filepath.Base(id)
	if clean != id {
		return "", fmt.Errorf("非法的文件名")
	}
	in, err := os.Open(filepath.Join(dir, clean))
	if err != nil {
		return "", fmt.Errorf("录制不存在")
	}
	defer in.Close()
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dl := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dl, 0755); err != nil {
		return "", err
	}
	dst := filepath.Join(dl, clean)
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(clean)
		stem := strings.TrimSuffix(clean, ext)
		dst = filepath.Join(dl, fmt.Sprintf("%s_%s%s", stem, time.Now().Format("150405"), ext))
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		// 中途失败（如磁盘满）不留半截损坏文件
		_ = out.Close()
		_ = os.Remove(dst)
		return "", fmt.Errorf("复制失败：%w", err)
	}
	return dst, nil
}

// DeleteRecording 删除录制文件（进行中的不允许删除）
func (a *App) DeleteRecording(id string) error {
	recMu.Lock()
	_, active := recActive[id]
	recMu.Unlock()
	if active {
		return fmt.Errorf("录制进行中，请先停止")
	}
	dir, err := recordingsDir()
	if err != nil {
		return err
	}
	clean := filepath.Base(id)
	if clean != id {
		return fmt.Errorf("非法的文件名")
	}
	return os.Remove(filepath.Join(dir, clean))
}
