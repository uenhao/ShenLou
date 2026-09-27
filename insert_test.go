package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestInsertStationAndUndo 验证插入与"删除后撤销恢复原序"
func TestInsertStationAndUndo(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Videos"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	a := NewApp()
	if err := a.CreatePlaylist("撤销测试"); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"A", "B", "C"} {
		if err := a.AddStation("撤销测试", Station{Name: n, URL: "http://x/" + n}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := a.GetStations("撤销测试")

	// 头部插入 → A? 变为 X,A,B,C
	if err := a.InsertStation("撤销测试", 0, Station{Name: "X", URL: "http://x/X"}); err != nil {
		t.Fatal(err)
	}
	got, _ := a.GetStations("撤销测试")
	if got[0].Name != "X" || len(got) != 4 {
		t.Errorf("头部插入错误：%v", got)
	}

	// 删除 B 再原位插回 → 与最初完全一致
	if err := a.RemoveStation("撤销测试", 2); err != nil {
		t.Fatal(err)
	}
	if err := a.InsertStation("撤销测试", 2, before[1]); err != nil {
		t.Fatal(err)
	}
	after, _ := a.GetStations("撤销测试")
	// after = X,A,B,C；截掉 X 后对比
	if !reflect.DeepEqual(after[1:], before) {
		t.Errorf("撤销恢复不一致：%v vs %v", after[1:], before)
	}

	// 越界位置自动追加到末尾
	if err := a.InsertStation("撤销测试", 99, Station{Name: "Z", URL: "http://x/Z"}); err != nil {
		t.Fatal(err)
	}
	got, _ = a.GetStations("撤销测试")
	if got[len(got)-1].Name != "Z" {
		t.Errorf("越界应追加到末尾：%v", got)
	}
}
