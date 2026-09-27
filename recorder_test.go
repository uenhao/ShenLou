package main

import (
	"testing"
	"time"
)

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"央广·中国之声":       "央广·中国之声",
		"a/b\\c":        "a_b_c",
		`Hit*FM?"live"`: "HitFM'live'",
		"":              "电台",
		"  ":            "电台",
		"<深夜>(电台)|版":    "(深夜)(电台)-版",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRecNameRe(t *testing.T) {
	m := recNameRe.FindStringSubmatch("测试·中国之声_20260927-101530.ts"[0:len("测试·中国之声_20260927-101530")])
	if m == nil {
		t.Fatal("应匹配 电台名_时间戳 格式")
	}
	if m[1] != "测试·中国之声" || m[2] != "20260927-101530" {
		t.Errorf("解析错误：%v", m)
	}
	// 带序号的去重文件名
	m2 := recNameRe.FindStringSubmatch("电台_20260927-101530_2")
	if m2 == nil || m2[1] != "电台" || m2[2] != "20260927-101530" {
		t.Errorf("带序号解析错误：%v", m2)
	}
	// 名字本身含下划线
	m3 := recNameRe.FindStringSubmatch("500首_经典_20260927-101530")
	if m3 == nil || m3[1] != "500首_经典" {
		t.Errorf("含下划线名称解析错误：%v", m3)
	}
	// 无时间戳的普通文件不匹配（按普通名字处理）
	if recNameRe.FindStringSubmatch("随便一个文件") != nil {
		t.Error("普通文件名不应匹配")
	}
}

func TestFmtDur(t *testing.T) {
	if got := fmtDur(65 * time.Second); got != "01:05" {
		t.Errorf("fmtDur(65s) = %q", got)
	}
	if got := fmtDur(3671 * time.Second); got != "1:01:11" {
		t.Errorf("fmtDur(3671s) = %q", got)
	}
}
