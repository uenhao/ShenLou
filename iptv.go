package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// TvChannel 电视台搜索结果：一个频道可带多路流（IPTV 源易失效，前端按序回退）
type TvChannel struct {
	Name       string   `json:"name"`
	Logo       string   `json:"logo"`
	Country    string   `json:"country"`
	Categories []string `json:"categories"`
	URLs       []string `json:"urls"`
}

// iptv-org 数据源（GitHub Pages 上的公开 API，全球频道库）
var iptvEndpoints = map[string]string{
	"channels": "https://iptv-org.github.io/api/channels.json",
	"streams":  "https://iptv-org.github.io/api/streams.json",
}

// 缓存 24 小时：channels+streams 合计约 5MB，没必要每次搜索都拉
const iptvCacheTTL = 24 * time.Hour

var (
	iptvMu       sync.Mutex
	iptvLoaded   bool
	iptvLoading  chan struct{} // 非 nil 表示一次加载正在进行；close 广播完成，全部等待者都会醒来
	iptvChannels []iptvChannel
	iptvURLs     map[string][]string // 频道 ID → 可播流地址（已过滤需特殊 header 的）
)

type iptvChannel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	AltNames   []string `json:"alt_names"`
	Country    string   `json:"country"`
	Categories []string `json:"categories"`
	IsNSFW     bool     `json:"is_nsfw"`
	Logo       string   `json:"logo"`
}

type iptvStream struct {
	Channel   string `json:"channel"`
	URL       string `json:"url"`
	Quality   string `json:"quality"`
	Referrer  string `json:"referrer"`
	UserAgent string `json:"user_agent"`
}

// iptvClient 双栈客户端（IPv4 优先：国内到 GitHub Pages 的 v6 链路不稳时自动换栈重试由上层负责）
func iptvClient(ipv4Only bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{MaxIdleConns: 4, IdleConnTimeout: 60 * time.Second}
	base := dialer.DialContext
	if ipv4Only {
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return base(ctx, "tcp4", addr)
		}
	} else {
		tr.DialContext = base
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: tr}
}

func iptvFetch(url string, ipv4Only bool) ([]byte, error) {
	client := iptvClient(ipv4Only)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ShenLou/1.0 (Wails; Linux)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 上限 64MB，防御异常响应
}

// iptvCachePath 数据目录下的缓存文件
func iptvCachePath(kind string) (string, error) {
	root, err := dataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cache", kind+".json"), nil
}

// loadIptvData 加载（必要时下载）频道与流数据。refresh=true 时强制重新下载。
// 下载在锁外进行（首载可达数十秒，持锁会把所有搜索请求排队卡死）；
// 并发调用经 iptvLoading 合并为一次下载：leader 完成后 close(done) 广播，
// 所有等待者都被唤醒（单值 channel 只能唤醒一个，等待者会永久挂起）。
func loadIptvData(refresh bool) error {
	iptvMu.Lock()
	if iptvLoaded && !refresh {
		iptvMu.Unlock()
		return nil
	}
	if iptvLoading != nil {
		ch := iptvLoading
		iptvMu.Unlock()
		<-ch
		// 醒来后按自身意图重判：leader 成功→loaded 返回 nil；
		// leader 失败或本调用是 refresh（在途的可能只是普通加载）→ 自己当 leader 再跑
		return loadIptvData(refresh)
	}
	done := make(chan struct{})
	iptvLoading = done
	iptvMu.Unlock()

	err := fetchIptvData(refresh)

	iptvMu.Lock()
	iptvLoading = nil
	iptvMu.Unlock()
	close(done)
	return err
}

// fetchIptvData 实际的下载/读缓存/解析/合并；仅在收尾发布时短暂持锁。
func fetchIptvData(refresh bool) error {
	root, err := dataRoot()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(root, "cache")
	_ = os.MkdirAll(cacheDir, 0755)

	get := func(kind string, out any) error {
		p := filepath.Join(cacheDir, kind+".json")
		useCache := false
		if !refresh {
			if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) < iptvCacheTTL {
				useCache = true
			}
		}
		var (
			data       []byte
			downloaded bool // 本次数据来自网络下载（而非缓存降级）
		)
		if useCache {
			if d, rerr := os.ReadFile(p); rerr == nil {
				data = d
			} else {
				useCache = false
			}
		}
		if !useCache {
			var ferr error
			// 先 IPv4 后双栈，与 52HzRadio 的经验一致
			for _, v4 := range []bool{true, false} {
				data, ferr = iptvFetch(iptvEndpoints[kind], v4)
				if ferr == nil {
					break
				}
			}
			if ferr != nil {
				// 下载失败但有过期缓存 → 降级用旧缓存
				if old, rerr := os.ReadFile(p); rerr == nil {
					data = old
				} else {
					return ferr
				}
			} else {
				downloaded = true
			}
		}
		if uerr := json.Unmarshal(data, out); uerr != nil && downloaded {
			// 下载拿到的是垃圾 body（空响应 / 劫持门户的 200 HTML）：
			// 绝不落盘污染缓存，降级旧缓存继续
			if old, rerr := os.ReadFile(p); rerr == nil && json.Unmarshal(old, out) == nil {
				data, downloaded = old, false
			} else {
				return fmt.Errorf("%s 数据解析失败：%w", kind, uerr)
			}
		} else if uerr != nil {
			return fmt.Errorf("%s 缓存数据解析失败：%w", kind, uerr)
		}
		if downloaded {
			// 只有"下载且解析成功"的数据才写缓存
			tmp := p + ".tmp"
			if os.WriteFile(tmp, data, 0644) == nil {
				_ = os.Rename(tmp, p)
			}
		}
		return nil
	}

	var channels []iptvChannel
	if err := get("channels", &channels); err != nil {
		return fmt.Errorf("频道库下载失败：%w", err)
	}
	var streams []iptvStream
	if err := get("streams", &streams); err != nil {
		return fmt.Errorf("流地址库下载失败：%w", err)
	}

	// 合并：频道 → 多路流（可用性优先：无需特殊 header 的排前面，高质量标签优先）
	urls := map[string][]string{}
	for _, s := range streams {
		if s.URL == "" || s.Channel == "" {
			continue
		}
		if s.Referrer != "" || s.UserAgent != "" {
			continue // 内嵌 <video> 无法带自定义 header，直接排除
		}
		urls[s.Channel] = append(urls[s.Channel], s.URL)
	}
	var merged []iptvChannel
	for _, ch := range channels {
		if ch.IsNSFW {
			continue
		}
		if len(urls[ch.ID]) == 0 {
			continue // 没有可播的流就没必要出现
		}
		merged = append(merged, ch)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })

	iptvMu.Lock()
	iptvChannels = merged
	iptvURLs = urls
	iptvLoaded = true
	iptvMu.Unlock()
	return nil
}

// SearchChannels 按名称/国家/分类搜索电视台。
// country 为 ISO 3166-1 alpha-2（如 CN）；category 如 news/music/general。
func (a *App) SearchChannels(name, country, category string, limit int) ([]TvChannel, error) {
	if limit <= 0 || limit > 300 {
		limit = 60
	}
	name = strings.TrimSpace(strings.ToLower(name))
	category = strings.TrimSpace(strings.ToLower(category))
	if err := loadIptvData(false); err != nil {
		return nil, err
	}
	iptvMu.Lock()
	defer iptvMu.Unlock()
	var out []TvChannel
	for _, ch := range iptvChannels {
		if country != "" && !strings.EqualFold(ch.Country, country) {
			continue
		}
		if category != "" {
			hit := false
			for _, c := range ch.Categories {
				if strings.EqualFold(c, category) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		if name != "" {
			hit := strings.Contains(strings.ToLower(ch.Name), name)
			if !hit {
				for _, alt := range ch.AltNames {
					if strings.Contains(strings.ToLower(alt), name) {
						hit = true
						break
					}
				}
			}
			if !hit {
				continue
			}
		}
		// 名称命中的排最前，其余按原（名称）序
		out = append(out, TvChannel{
			Name:       ch.Name,
			Logo:       ch.Logo,
			Country:    ch.Country,
			Categories: ch.Categories,
			URLs:       append([]string(nil), iptvURLs[ch.ID]...),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// IptvCategories 返回全部可用分类（前端下拉）
func (a *App) IptvCategories() ([]string, error) {
	if err := loadIptvData(false); err != nil {
		return nil, err
	}
	iptvMu.Lock()
	defer iptvMu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, ch := range iptvChannels {
		for _, c := range ch.Categories {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// RefreshIptv 强制刷新数据源缓存
func (a *App) RefreshIptv() error {
	iptvMu.Lock()
	iptvLoaded = false
	iptvMu.Unlock()
	return loadIptvData(true)
}
