package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMoveStationTo 验证拖拽排序的后端移动逻辑（隔离 HOME）
func TestMoveStationTo(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	a := NewApp()
	if err := a.CreatePlaylist("测试列表"); err != nil {
		t.Fatal(err)
	}
	names := []string{"甲", "乙", "丙", "丁"}
	for _, n := range names {
		if err := a.AddStation("测试列表", Station{Name: n, URL: "http://x/" + n}); err != nil {
			t.Fatal(err)
		}
	}
	order := func() []string {
		sts, _ := a.GetStations("测试列表")
		out := []string{}
		for _, s := range sts {
			out = append(out, s.Name)
		}
		return out
	}

	// 首位移到末位：甲乙丙丁 → 乙丙丁甲
	if err := a.MoveStationTo("测试列表", 0, 3); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != "乙" || got[3] != "甲" {
		t.Errorf("首→末 失败：%v", got)
	}
	// 末位移回首：乙丙丁甲 → 甲乙丙丁
	if err := a.MoveStationTo("测试列表", 3, 0); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != "甲" || got[3] != "丁" {
		t.Errorf("末→首 失败：%v", got)
	}
	// 中间移动：甲乙丙丁 → 甲丙乙丁
	if err := a.MoveStationTo("测试列表", 1, 2); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[1] != "丙" || got[2] != "乙" {
		t.Errorf("中间移动失败：%v", got)
	}
	// 越界报错
	if err := a.MoveStationTo("测试列表", 0, 9); err == nil {
		t.Error("越界应报错")
	}
	// 原地不动
	if err := a.MoveStationTo("测试列表", 1, 1); err != nil {
		t.Error("原地移动不应报错")
	}
}
