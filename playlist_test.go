package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteM3URoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.m3u")

	original := []Station{
		{Name: "中国之声", URL: "http://ngcdn001.cnr.cn/live/zgzs/index.m3u8"},
		{Name: "经济之声", URL: "http://ngcdn002.cnr.cn/live/jjzs/index.m3u8"},
	}
	if err := writeM3U(path, original); err != nil {
		t.Fatalf("writeM3U: %v", err)
	}
	got, err := readM3U(path)
	if err != nil {
		t.Fatalf("readM3U: %v", err)
	}
	if len(got) != len(original) {
		t.Fatalf("条目数不符：got %d want %d", len(got), len(original))
	}
	for i := range original {
		if got[i] != original[i] {
			t.Errorf("第 %d 条不符：got %+v want %+v", i, got[i], original[i])
		}
	}
}

func TestReadM3UParsesExistingFile(t *testing.T) {
	// 模拟用户现有的 m3u（含空行、无名条目、多余注释）
	content := "#EXTM3U\n\n#EXTINF:-1,央广·音乐之声\nhttp://ngcdn001.cnr.cn/live/yyzs/index.m3u8\n#EXTINF:-1,华语金曲500首\nhttp://ls.qingting.fm/live/3412131.m3u8?bitrate=64\nhttp://example.com/plain-stream\n#comment\n"
	path := filepath.Join(t.TempDir(), "x.m3u")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readM3U(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("应解析出 3 个电台，got %d", len(got))
	}
	if got[0].Name != "央广·音乐之声" || got[1].Name != "华语金曲500首" {
		t.Errorf("名称解析错误：%+v", got)
	}
	if got[2].Name != got[2].URL {
		t.Errorf("无 EXTINF 的条目应以地址为名：%+v", got[2])
	}
}

func TestValidName(t *testing.T) {
	for _, bad := range []string{"", "  ", "a/b", "a\\b", "..", "../x", ".hidden"} {
		if _, err := validName(bad); err == nil {
			t.Errorf("应拒绝非法名称 %q", bad)
		}
	}
	for _, ok := range []string{"我的电台", "favorites-2026", "My List (中文)"} {
		if _, err := validName(ok); err != nil {
			t.Errorf("应接受合法名称 %q: %v", ok, err)
		}
	}
}

func TestNormalizeStation(t *testing.T) {
	st, err := normalizeStation(Station{Name: "  测试台 ", URL: "example.com/stream"})
	if err != nil {
		t.Fatal(err)
	}
	if st.URL != "http://example.com/stream" {
		t.Errorf("应自动补协议前缀，got %s", st.URL)
	}
	if !strings.Contains(st.Name, "测试台") {
		t.Errorf("名称应去除空白，got %q", st.Name)
	}
	if _, err := normalizeStation(Station{Name: "x", URL: " "}); err == nil {
		t.Error("空地址应报错")
	}
	// 换行/控制字符可向 m3u 注入额外条目，必须拒绝
	for _, bad := range []string{
		"http://a.cn/x\n#EXTINF:-1,evil\nhttp://evil",
		"http://a.cn/x\r\nhttp://evil",
		"http://a.cn/\tx",
	} {
		if _, err := normalizeStation(Station{Name: "x", URL: bad}); err == nil {
			t.Errorf("含控制字符的地址应报错：%q", bad)
		}
	}
	// 名称超长截断到 60 字符
	stLong, errLong := normalizeStation(Station{Name: strings.Repeat("台", 80), URL: "http://x"})
	if errLong != nil {
		t.Fatal(errLong)
	}
	if got := len([]rune(stLong.Name)); got != 60 {
		t.Errorf("名称应截断到 60 字符，got %d", got)
	}
}
