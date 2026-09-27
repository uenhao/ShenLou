package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestToggleFavorite 验证收藏切换：加入、重复识别、移出（隔离 HOME）
func TestToggleFavorite(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	a := NewApp()
	st := Station{Name: "测试电台", URL: "http://example.com/live"}

	added, err := a.ToggleFavorite(st)
	if err != nil || !added {
		t.Fatalf("首次应加入收藏：%v added=%v", err, added)
	}
	urls, _ := a.FavoriteURLs()
	if len(urls) != 1 || urls[0] != st.URL {
		t.Fatalf("收藏列表内容不符：%v", urls)
	}

	// 同一电台再切一次 → 移出
	added, err = a.ToggleFavorite(st)
	if err != nil || added {
		t.Fatalf("再次切换应移出：%v added=%v", err, added)
	}
	urls, _ = a.FavoriteURLs()
	if len(urls) != 0 {
		t.Fatalf("移出后应为空：%v", urls)
	}

	// 收藏列表被删后（用户手动删除），切换收藏应能自动重建
	if root, err := dataRoot(); err == nil {
		_ = os.Remove(filepath.Join(root, favoritePlaylistName+".m3u"))
	}
	added, err = a.ToggleFavorite(st)
	if err != nil || !added {
		t.Fatalf("列表缺失时应自动重建并加入：%v added=%v", err, added)
	}
}
