package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFavoriteNotAPlaylist 收藏不再作为播放列表：不出现在列表中、不可建/删/改名、FindStation 不把它算作归属
func TestFavoriteNotAPlaylist(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	a := NewApp()
	// 收藏两个电台，其中一个同时放进普通播放列表
	if _, err := a.ToggleFavorite(Station{Name: "台A", URL: "http://a.cn/x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ToggleFavorite(Station{Name: "台B", URL: "http://b.cn/x"}); err != nil {
		t.Fatal(err)
	}
	if err := a.CreatePlaylist("列表一"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddStation("列表一", Station{Name: "台A", URL: "http://a.cn/x"}); err != nil {
		t.Fatal(err)
	}

	// 列表中只有「列表一」
	pls, err := a.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(pls) != 1 || pls[0].Name != "列表一" {
		t.Fatalf("播放列表应只含「列表一」：%v", pls)
	}

	// 防护：不能对收藏名建/删/改
	if err := a.CreatePlaylist(favoritePlaylistName); err == nil {
		t.Error("收藏名不可用于新建播放列表")
	}
	if err := a.DeletePlaylist(favoritePlaylistName); err == nil {
		t.Error("收藏记录不可删除")
	}
	if err := a.RenamePlaylist("列表一", favoritePlaylistName); err == nil {
		t.Error("播放列表不可改名为收藏名")
	}
	if err := a.AddStation(favoritePlaylistName, Station{Name: "x", URL: "http://c.cn/x"}); err == nil {
		t.Error("收藏不可通过 AddStation 添加")
	}

	// FavoriteStations：台A 带「列表一」归属；台B 无归属
	favs, err := a.FavoriteStations()
	if err != nil {
		t.Fatal(err)
	}
	if len(favs) != 2 {
		t.Fatalf("应有 2 条收藏：%v", favs)
	}
	byURL := map[string]FavStation{}
	for _, f := range favs {
		byURL[f.URL] = f
	}
	if got := byURL["http://a.cn/x"].Playlists; len(got) != 1 || got[0] != "列表一" {
		t.Errorf("台A 归属应为 [列表一]：%v", got)
	}
	if got := byURL["http://b.cn/x"].Playlists; len(got) != 0 {
		t.Errorf("台B 不应归属任何列表：%v", got)
	}

	// FindStation 不把收藏本身算作归属
	owner, err := a.FindStation("http://b.cn/x")
	if err != nil {
		t.Fatal(err)
	}
	if len(owner) != 0 {
		t.Errorf("台B 不在任何播放列表：%v", owner)
	}
}

// TestPlayHistory 历史记录：去重置顶、上限、清空
func TestPlayHistory(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	a := NewApp()
	a.recordHistory("台1", "http://1.cn/x")
	a.recordHistory("台2", "http://2.cn/x")
	a.recordHistory("台1", "http://1.cn/x") // 去重置顶

	h := a.PlayHistory()
	if len(h) != 2 || h[0].URL != "http://1.cn/x" || h[1].URL != "http://2.cn/x" {
		t.Fatalf("历史应去重且最近优先：%v", h)
	}

	// 本地文件回听不入历史
	a.recordHistory("回听", "/home/x/recordings/a.ts")
	if got := len(a.PlayHistory()); got != 2 {
		t.Fatalf("本地文件不应入历史：%d", got)
	}

	// 上限 100
	for i := 0; i < 150; i++ {
		a.recordHistory("台", "http://x.cn/"+string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	if got := len(a.PlayHistory()); got != historyMax {
		t.Fatalf("历史应封顶 %d：%d", historyMax, got)
	}

	// 持久化 + 重启加载 + 清空
	a2 := NewApp()
	a2.loadHistory()
	if got := len(a2.PlayHistory()); got != historyMax {
		t.Fatalf("重启后应从 history.json 恢复：%d", got)
	}
	if err := a2.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if got := len(a2.PlayHistory()); got != 0 {
		t.Fatalf("清空后应为空：%d", got)
	}
	a3 := NewApp()
	a3.loadHistory()
	if got := len(a3.PlayHistory()); got != 0 {
		t.Fatalf("清空后文件应已删除：%d", got)
	}
}

// TestListRecordingsActiveNoFile 活跃录制在 .ts 落盘前也必须出现在列表中
// （否则前端轮询对比会把启动初期的录制误判为异常结束）
func TestListRecordingsActiveNoFile(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	a := NewApp()
	id := "测试台_20260927-153000.ts"
	recMu.Lock()
	recActive[id] = &recordingProc{url: "http://x.cn/live", start: time.Now()}
	recMu.Unlock()
	defer func() {
		recMu.Lock()
		delete(recActive, id)
		recMu.Unlock()
	}()

	list, err := a.ListRecordings()
	if err != nil {
		t.Fatal(err)
	}
	var hit *RecordingInfo
	for i := range list {
		if list[i].ID == id {
			hit = &list[i]
		}
	}
	if hit == nil {
		t.Fatalf("未落盘的活跃录制应出现在列表中：%v", list)
	}
	if !hit.Active || hit.Name != "测试台" || hit.URL != "http://x.cn/live" {
		t.Fatalf("合成条目字段不符：%+v", hit)
	}
}

// TestEnsureBuiltinPlaylists 内置「默认列表」：仅 CCTV-8K；旧「中文频道」迁移；不可删改
func TestEnsureBuiltinPlaylists(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	// 旧版内置文件存在 → 被迁移清除，新默认列表只含 CCTV-8K
	root := filepath.Join(home, "Videos", ".ShenLou")
	_ = os.MkdirAll(root, 0755)
	if err := os.WriteFile(filepath.Join(root, "中文频道.m3u"), []byte("#EXTM3U\n#EXTINF:-1,旧台\nhttp://old/x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ensureBuiltinPlaylists()
	if fileExists(filepath.Join(root, "中文频道.m3u")) {
		t.Fatal("旧「中文频道」应被迁移清除")
	}
	sts, err := readM3U(filepath.Join(root, "默认列表.m3u"))
	if err != nil || len(sts) != 1 || sts[0].Name != "CCTV-8K" {
		t.Fatalf("默认列表应只含 CCTV-8K：%v %v", err, sts)
	}

	// 用户整理过 → 不覆盖
	if err := os.WriteFile(filepath.Join(root, "默认列表.m3u"), []byte("#EXTM3U\n#EXTINF:-1,我的\nhttp://a/x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ensureBuiltinPlaylists()
	sts, _ = readM3U(filepath.Join(root, "默认列表.m3u"))
	if len(sts) != 1 || sts[0].Name != "我的" {
		t.Fatalf("用户整理过的默认列表不应被覆盖：%v", sts)
	}

	// 防护：默认列表不可删/改名；收藏列表同理
	a := NewApp()
	if err := a.DeletePlaylist(builtinPlaylistName); err == nil {
		t.Error("默认列表不可删除")
	}
	if err := a.RenamePlaylist(builtinPlaylistName, "xx"); err == nil {
		t.Error("默认列表不可改名")
	}
	if err := a.CreatePlaylist(builtinPlaylistName); err == nil {
		t.Error("默认列表名不可占用")
	}

	// 跨列表迁移 + 收藏/历史引用联动（按 URL 关联，迁移后 FindStation 指向新列表）
	if err := a.CreatePlaylist("列表A"); err != nil {
		t.Fatal(err)
	}
	if err := a.CreatePlaylist("列表B"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddStation("列表A", Station{Name: "迁移台", URL: "http://mv/x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ToggleFavorite(Station{Name: "迁移台", URL: "http://mv/x"}); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveStationCross("列表A", 0, "列表B"); err != nil {
		t.Fatal(err)
	}
	owner, _ := a.FindStation("http://mv/x")
	if len(owner) != 1 || owner[0] != "列表B" {
		t.Fatalf("迁移后归属应为列表B：%v", owner)
	}
	// 目标已含同 URL：仅从来源移除
	if err := a.AddStation("列表A", Station{Name: "迁移台", URL: "http://mv/x"}); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveStationCross("列表A", 0, "列表B"); err != nil {
		t.Fatal(err)
	}
	stsA, _ := a.GetStations("列表A")
	stsB, _ := a.GetStations("列表B")
	if len(stsA) != 0 || len(stsB) != 1 {
		t.Fatalf("去重迁移结果不符：A=%v B=%v", stsA, stsB)
	}
}
