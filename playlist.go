package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var plMu sync.Mutex

// 内置播放列表数据（首次启动写入数据目录，用户已有同名文件则不覆盖）
//
// builtinPlaylistName 内置默认列表（不可删除/重命名；搜索播放的频道自动落入这里）
const builtinPlaylistName = "默认列表"

//go:embed builtin/default.m3u
var builtinDefaultM3U []byte

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ensureBuiltinPlaylists 确保内置「默认列表」（只含 CCTV-8K）存在；
// 旧版内置「中文频道」（自动生成的 60 台清单）升格迁移为默认列表并重置内容。
func ensureBuiltinPlaylists() {
	plMu.Lock()
	defer plMu.Unlock()
	root, err := dataRoot()
	if err != nil {
		return
	}
	if old := filepath.Join(root, "中文频道.m3u"); fileExists(old) {
		_ = os.Remove(old) // 旧内容为内置自动生成，非用户数据；新内置仅保留 CCTV-8K
	}
	p := filepath.Join(root, builtinPlaylistName+".m3u")
	if fileExists(p) {
		return // 用户已整理过默认列表，不覆盖
	}
	_ = os.WriteFile(p, builtinDefaultM3U, 0644)
}

// PlaylistInfo 播放列表摘要（左侧列表用）
type PlaylistInfo struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Count int    `json:"count"`
}

// dataRoot 应用数据目录：~/Videos/.ShenLou（频道列表与录制文件均存放于此），不存在则自动创建
func dataRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	videos := filepath.Join(home, "Videos")
	_ = os.MkdirAll(videos, 0755) // 部分精简安装可能没有 Videos，直接补建
	root := filepath.Join(videos, ".ShenLou")
	if err := os.MkdirAll(filepath.Join(root, "recordings"), 0755); err != nil {
		return "", err
	}
	return root, nil
}

// migrateOldDataRoot 一次性迁移：旧目录 ~/Music/.radiohub → ~/Music/.ShenLou。

// favoritePlaylistName 内置收藏播放列表名
const favoritePlaylistName = "我的收藏"

// ensureFavoriteList 确保内置收藏列表存在
func ensureFavoriteList() {
	plMu.Lock()
	defer plMu.Unlock()
	root, err := dataRoot()
	if err != nil {
		return
	}
	p := filepath.Join(root, favoritePlaylistName+".m3u")
	if _, err := os.Stat(p); err != nil {
		_ = os.WriteFile(p, []byte("#EXTM3U\n"), 0644)
	}
}

// FavoriteURLs 返回收藏列表中全部流地址（前端标记 ⭐ 用）
func (a *App) FavoriteURLs() ([]string, error) {
	plMu.Lock()
	defer plMu.Unlock()
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}
	sts, err := readM3U(filepath.Join(root, favoritePlaylistName+".m3u"))
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(sts))
	for _, s := range sts {
		urls = append(urls, s.URL)
	}
	return urls, nil
}

// ToggleFavorite 切换电台的收藏状态：返回 true=已加入，false=已移出
func (a *App) ToggleFavorite(st Station) (bool, error) {
	path, err := playlistPath(favoritePlaylistName)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 收藏存储缺失时直接重建（不走 CreatePlaylist——该名称已被防护占用）
		if err := os.WriteFile(path, []byte("#EXTM3U\n"), 0644); err != nil {
			return false, err
		}
	}
	var added bool
	err = a.mutateStations(favoritePlaylistName, func(sts []Station) ([]Station, error) {
		nst, err := normalizeStation(st) // 与其它入口一致：拒绝换行注入、补全协议、截断名字
		if err != nil {
			return nil, err
		}
		for i, s := range sts {
			if s.URL == nst.URL {
				added = false
				return append(sts[:i], sts[i+1:]...), nil
			}
		}
		added = true
		return append(sts, nst), nil
	})
	return added, err
}

// PlaylistsDir 前端展示目录位置用
func (a *App) PlaylistsDir() string {
	dir, err := dataRoot()
	if err != nil {
		return ""
	}
	return dir
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("名称不能为空")
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("名称不能包含 / \\ .. 或以点开头")
	}
	if len(name) > 100 {
		return "", fmt.Errorf("名称过长（最多 100 字符）")
	}
	return name, nil
}

func playlistPath(name string) (string, error) {
	n, err := validName(name)
	if err != nil {
		return "", err
	}
	root, err := dataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, n+".m3u"), nil
}

// readM3U 解析扩展 m3u：#EXTINF 行取逗号后的名称，其后第一个非注释行为地址
func readM3U(path string) ([]Station, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var stations []Station
	pending := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXTINF") {
			if i := strings.Index(line, ","); i >= 0 {
				pending = strings.TrimSpace(line[i+1:])
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		name := pending
		if name == "" {
			name = line
		}
		stations = append(stations, Station{Name: name, URL: line})
		pending = ""
	}
	return stations, nil
}

// writeM3U 原子写回，防止中途失败损坏文件
func writeM3U(path string, stations []Station) error {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, st := range stations {
		name := strings.ReplaceAll(st.Name, "\n", " ")
		if name == "" {
			name = st.URL
		}
		fmt.Fprintf(&b, "#EXTINF:-1,%s\n%s\n", name, st.URL)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// playlistFiles 列出数据目录中的 m3u 文件名（不含内置收藏文件：
// 收藏是独立记录列表，不再作为普通播放列表展示或参与"所属列表"归属）
func playlistFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	fav := favoritePlaylistName + ".m3u"
	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".m3u" {
			continue
		}
		if e.Name() == fav {
			continue
		}
		files = append(files, e.Name())
	}
	return files
}

// BuiltinPlaylistName 内置默认列表名（前端与其硬编码字符串保持单一真值）
func (a *App) BuiltinPlaylistName() string {
	return builtinPlaylistName
}

// ListPlaylists 列出所有播放列表及其电台数（不含收藏）
func (a *App) ListPlaylists() ([]PlaylistInfo, error) {
	plMu.Lock()
	defer plMu.Unlock()
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}
	var list []PlaylistInfo
	for _, f := range playlistFiles(root) {
		sts, err := readM3U(filepath.Join(root, f))
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(f, filepath.Ext(f))
		list = append(list, PlaylistInfo{Name: name, File: f, Count: len(sts)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// FavStation 收藏条目：电台 + 它所在的播放列表名（前端定位跳转用）
type FavStation struct {
	Station
	Playlists []string `json:"playlists"`
}

// FavoriteStations 收藏记录（独立视图），附带每条所在播放列表
func (a *App) FavoriteStations() ([]FavStation, error) {
	plMu.Lock()
	defer plMu.Unlock()
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}
	favs, err := readM3U(filepath.Join(root, favoritePlaylistName+".m3u"))
	if err != nil {
		return nil, err
	}
	owner := map[string][]string{}
	for _, f := range playlistFiles(root) {
		sts, err := readM3U(filepath.Join(root, f))
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(f, filepath.Ext(f))
		for _, st := range sts {
			owner[st.URL] = append(owner[st.URL], name)
		}
	}
	out := make([]FavStation, 0, len(favs))
	for _, f := range favs {
		out = append(out, FavStation{Station: f, Playlists: owner[f.URL]})
	}
	return out, nil
}

// CreatePlaylist 新建空播放列表（收藏是内置记录，不占用此名称）
func (a *App) CreatePlaylist(name string) error {
	plMu.Lock()
	defer plMu.Unlock()
	if name == favoritePlaylistName || name == builtinPlaylistName {
		return fmt.Errorf("「%s」是内置列表，不能作为频道列表名称", name)
	}
	path, err := playlistPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("播放列表已存在：%s", name)
	}
	return os.WriteFile(path, []byte("#EXTM3U\n"), 0644)
}

// RenamePlaylist 重命名播放列表文件（内置收藏不可改名）
func (a *App) RenamePlaylist(oldName, newName string) error {
	plMu.Lock()
	defer plMu.Unlock()
	for _, n := range []string{oldName, newName} {
		if n == favoritePlaylistName || n == builtinPlaylistName {
			return fmt.Errorf("「%s」是内置列表，不能重命名或占用此名称", n)
		}
	}
	oldPath, err := playlistPath(oldName)
	if err != nil {
		return err
	}
	newPath, err := playlistPath(newName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldPath); err != nil {
		return fmt.Errorf("播放列表不存在：%s", oldName)
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("目标名称已存在：%s", newName)
	}
	return os.Rename(oldPath, newPath)
}

// DeletePlaylist 删除播放列表文件（内置收藏不可删除）
func (a *App) DeletePlaylist(name string) error {
	plMu.Lock()
	defer plMu.Unlock()
	if name == favoritePlaylistName || name == builtinPlaylistName {
		return fmt.Errorf("「%s」是内置列表，不能删除", name)
	}
	path, err := playlistPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("播放列表不存在：%s", name)
	}
	return os.Remove(path)
}

// GetStations 读取播放列表的全部电台
func (a *App) GetStations(name string) ([]Station, error) {
	plMu.Lock()
	defer plMu.Unlock()
	path, err := playlistPath(name)
	if err != nil {
		return nil, err
	}
	return readM3U(path)
}

// FindStation 返回包含指定流地址的播放列表名列表（不含收藏；收藏条目定位/归属用）
func (a *App) FindStation(url string) ([]string, error) {
	plMu.Lock()
	defer plMu.Unlock()
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("地址为空")
	}
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}
	var found []string
	for _, f := range playlistFiles(root) {
		sts, err := readM3U(filepath.Join(root, f))
		if err != nil {
			continue
		}
		for _, st := range sts {
			if st.URL == url {
				found = append(found, strings.TrimSuffix(f, filepath.Ext(f)))
				break
			}
		}
	}
	return found, nil
}

// mutateStations 读取-修改-写回的统一入口，串行化防止并发写坏文件
func (a *App) mutateStations(name string, fn func(sts []Station) ([]Station, error)) error {
	plMu.Lock()
	defer plMu.Unlock()
	path, err := playlistPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("播放列表不存在：%s", name)
	}
	sts, err := readM3U(path)
	if err != nil {
		return err
	}
	sts, err = fn(sts)
	if err != nil {
		return err
	}
	return writeM3U(path, sts)
}

func normalizeStation(st Station) (Station, error) {
	st.Name = strings.TrimSpace(st.Name)
	st.URL = strings.TrimSpace(st.URL)
	if st.URL == "" {
		return st, fmt.Errorf("频道地址不能为空")
	}
	// 换行/控制字符会破坏 m3u 的行结构（可注入额外条目），直接拒绝
	if strings.ContainsAny(st.URL, "\r\n\t") {
		return st, fmt.Errorf("频道地址不能包含换行或控制字符")
	}
	if !strings.Contains(st.URL, "://") {
		st.URL = "http://" + st.URL
	}
	if st.Name == "" {
		st.Name = st.URL
	}
	if r := []rune(st.Name); len(r) > 60 {
		st.Name = string(r[:60])
	}
	return st, nil
}

// AddStation 向播放列表追加一个电台（手动添加/搜索结果收藏共用；收藏请走 ToggleFavorite）
func (a *App) AddStation(pl string, st Station) error {
	if pl == favoritePlaylistName {
		return fmt.Errorf("收藏请使用星标按钮，不作为播放列表添加")
	}
	return a.mutateStations(pl, func(sts []Station) ([]Station, error) {
		nst, err := normalizeStation(st)
		if err != nil {
			return nil, err
		}
		for _, s := range sts {
			if s.URL == nst.URL {
				return nil, fmt.Errorf("该频道已在列表中：%s", nst.Name)
			}
		}
		return append(sts, nst), nil
	})
}

// UpdateStation 修改指定序号的电台（收藏由 ToggleFavorite 管理）
func (a *App) UpdateStation(pl string, idx int, st Station) error {
	if pl == favoritePlaylistName {
		return fmt.Errorf("收藏记录由星标按钮管理，不能直接编辑")
	}
	return a.mutateStations(pl, func(sts []Station) ([]Station, error) {
		if idx < 0 || idx >= len(sts) {
			return nil, fmt.Errorf("序号超出范围")
		}
		nst, err := normalizeStation(st)
		if err != nil {
			return nil, err
		}
		sts[idx] = nst
		return sts, nil
	})
}

// RemoveStation 删除指定序号的电台
func (a *App) RemoveStation(pl string, idx int) error {
	if pl == favoritePlaylistName {
		return fmt.Errorf("收藏由星标管理，不能直接改写「%s」", favoritePlaylistName)
	}
	return a.mutateStations(pl, func(sts []Station) ([]Station, error) {
		if idx < 0 || idx >= len(sts) {
			return nil, fmt.Errorf("序号超出范围")
		}
		return append(sts[:idx], sts[idx+1:]...), nil
	})
}

// InsertStation 在指定位置插入电台（撤销删除用；idx 越界时追加到末尾）
func (a *App) InsertStation(pl string, idx int, st Station) error {
	if pl == favoritePlaylistName {
		return fmt.Errorf("收藏由星标管理，不能直接改写「%s」", favoritePlaylistName)
	}
	return a.mutateStations(pl, func(sts []Station) ([]Station, error) {
		nst, err := normalizeStation(st)
		if err != nil {
			return nil, err
		}
		if idx < 0 || idx > len(sts) {
			idx = len(sts)
		}
		out := make([]Station, 0, len(sts)+1)
		out = append(out, sts[:idx]...)
		out = append(out, nst)
		out = append(out, sts[idx:]...)
		return out, nil
	})
}

// MoveStationTo 把 from 序号的电台移动到 to 序号位置（拖拽排序用）
func (a *App) MoveStationTo(pl string, from, to int) error {
	return a.mutateStations(pl, func(sts []Station) ([]Station, error) {
		if from < 0 || from >= len(sts) || to < 0 || to >= len(sts) {
			return nil, fmt.Errorf("序号超出范围")
		}
		if from == to {
			return sts, nil
		}
		moved := sts[from]
		result := make([]Station, 0, len(sts))
		result = append(result, sts[:from]...)
		result = append(result, sts[from+1:]...)
		out := make([]Station, 0, len(sts))
		out = append(out, result[:to]...)
		out = append(out, moved)
		out = append(out, result[to:]...)
		return out, nil
	})
}

// MoveStationCross 把一个频道从 from 列表迁移到 to 列表（目标已有时仅从来源移除）。
// 频道按 URL 关联收藏/历史，迁移后它们的"所属列表"引用自动指向新列表。
func (a *App) MoveStationCross(from string, idx int, to string) error {
	if from == favoritePlaylistName || to == favoritePlaylistName {
		return fmt.Errorf("收藏由星标管理，不参与迁移")
	}
	if from == to {
		return fmt.Errorf("来源与目标列表相同") // 自迁移会在去重分支把频道直接删掉
	}
	plMu.Lock()
	defer plMu.Unlock()
	fromPath, err := playlistPath(from)
	if err != nil {
		return err
	}
	toPath, err := playlistPath(to)
	if err != nil {
		return err
	}
	if !fileExists(fromPath) || !fileExists(toPath) {
		return fmt.Errorf("频道列表不存在")
	}
	src, err := readM3U(fromPath)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(src) {
		return fmt.Errorf("序号超出范围")
	}
	st := src[idx]
	dst, err := readM3U(toPath)
	if err != nil {
		return err
	}
	removed := append(src[:idx], src[idx+1:]...)
	for _, d := range dst {
		if d.URL == st.URL {
			return writeM3U(fromPath, removed)
		}
	}
	// 先写目标再删来源：中途失败最多留下重复项（可恢复），不会丢频道
	if err := writeM3U(toPath, append(dst, st)); err != nil {
		return err
	}
	return writeM3U(fromPath, removed)
}
