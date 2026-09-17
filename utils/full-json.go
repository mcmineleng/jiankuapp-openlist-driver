package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const baseURL = "https://www.jiankuapp.com/apps"

// 列表页卡片里的详情链接：/app/detail?id=563
var detailRe = regexp.MustCompile(`/app/detail\?id=(\d+)`)

// 详情页里 ?id=N
var queryIDRe = regexp.MustCompile(`[?&]id=(\d+)`)

// 提取下载域名作源名
var hostRe = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$`)

// 新增：匹配 "第 1/19 页" 中的总页数
var pageCountRe = regexp.MustCompile(`第\s*\d+\s*/\s*(\d+)\s*页`)

// 新增：匹配 "共找到 544 个相关资源"
var totalCountRe = regexp.MustCompile(`共找到\s*(\d+)\s*个相关资源`)

type item struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type app struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Downloads   string `json:"downloads"`
	Icon        string `json:"icon"`
	DetailURL   string `json:"detail_url"`
	Description string `json:"description"`
	Sources     []item `json:"sources"`
}

type result struct {
	Query      string `json:"query"`
	ReqURL     string `json:"request_url"`
	Page       int    `json:"page"`
	TotalPages int    `json:"total_pages"`   // 新增：总页数
	TotalItems int    `json:"total_items"`   // 新增：总资源数
	HasNext    bool   `json:"has_next"`
	Apps       []app  `json:"apps"`
}

// 新增：多页抓取结果
type multiResult struct {
	Query      string   `json:"query"`
	TotalPages int      `json:"total_pages"`
	TotalItems int      `json:"total_items"`
	Pages      []result `json:"pages"`
	AllApps    []app    `json:"all_apps"`
}

func main() {
	q := flag.String("q", "", "搜索关键词，留空表示获取全部资源")
	page := flag.Int("page", 1, "页码，从 1 开始")
	detail := flag.Bool("detail", true, "是否跟进详情页抓取描述与下载源")
	timeout := flag.Duration("timeout", 25*time.Second, "HTTP 请求超时时间")
	all := flag.Bool("all", false, "是否抓取所有页面（自动从第一页开始，直到最后一页）")
	flag.Parse()

	if *all {
		// 全量抓取模式
		multi := scrapeAll(*q, *detail, *timeout)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(multi); err != nil {
			fatal("输出 JSON 失败: %v", err)
		}
		return
	}

	// 单页模式（原逻辑）
	reqURL := buildURL(*q, *page)

	doc, err := fetch(reqURL, *timeout)
	if err != nil {
		fatal("请求失败: %v", err)
	}

	res := parsePage(doc, *q, reqURL, *page, *detail, *timeout)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(res); err != nil {
		fatal("输出 JSON 失败: %v", err)
	}
}

// 新增：全量抓取所有页面
func scrapeAll(q string, detail bool, timeout time.Duration) multiResult {
	multi := multiResult{
		Query: strings.TrimSpace(q),
		Pages: []result{},
		AllApps: []app{},
	}

	// 先抓第一页，获取总页数
	firstURL := buildURL(q, 1)
	doc, err := fetch(firstURL, timeout)
	if err != nil {
		fatal("请求失败: %v", err)
	}

	firstPage := parsePage(doc, q, firstURL, 1, detail, timeout)
	multi.TotalPages = firstPage.TotalPages
	multi.TotalItems = firstPage.TotalItems
	multi.Pages = append(multi.Pages, firstPage)
	multi.AllApps = append(multi.AllApps, firstPage.Apps...)

	log.Printf("共找到 %d 个资源，共 %d 页", multi.TotalItems, multi.TotalPages)

	// 从第 2 页开始抓取
	for p := 2; p <= multi.TotalPages; p++ {
		time.Sleep(1 * time.Second) // 礼貌延迟
		pageURL := buildURL(q, p)
		log.Printf("正在抓取第 %d/%d 页: %s", p, multi.TotalPages, pageURL)

		doc, err := fetch(pageURL, timeout)
		if err != nil {
			log.Printf("第 %d 页抓取失败: %v", p, err)
			continue
		}

		pageResult := parsePage(doc, q, pageURL, p, detail, timeout)
		multi.Pages = append(multi.Pages, pageResult)
		multi.AllApps = append(multi.AllApps, pageResult.Apps...)

		log.Printf("第 %d 页完成，本页 %d 个资源，累计 %d 个",
			p, len(pageResult.Apps), len(multi.AllApps))
	}

	return multi
}

// 新增：解析单个页面（从原 main 中提取）
func parsePage(doc *goquery.Document, q, reqURL string, page int, detail bool, timeout time.Duration) result {
	res := result{
		Query:  strings.TrimSpace(q),
		ReqURL: reqURL,
		Page:   page,
		Apps:   []app{},
	}

	// 提取总页数和总资源数
	bodyText := doc.Text()

	if m := pageCountRe.FindStringSubmatch(bodyText); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			res.TotalPages = n
		}
	}
	if m := totalCountRe.FindStringSubmatch(bodyText); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			res.TotalItems = n
		}
	}

	// 分页：存在"下一页"链接即认为还有下一页
	if doc.Find("a[rel=next], a:contains('下一页'), a:contains('»')").Length() > 0 {
		res.HasNext = true
	}
	// 如果没解析到总页数，用 HasNext 兜底
	if res.TotalPages == 0 && res.HasNext {
		res.TotalPages = page + 1
	}

	doc.Find("div.glass-card.app-card").Each(func(_ int, card *goquery.Selection) {
		link := card.Find("a").First()
		href, _ := link.Attr("href")
		id := ""
		if m := detailRe.FindStringSubmatch(href); len(m) == 2 {
			id = m[1]
		}

		title := strings.TrimSpace(link.Find("h3").First().Text())

		metaDiv := link.Find("h3").First().Parent().Find("div.flex.items-center")
		spans := metaDiv.Find("span")
		category, version, downloads := "", "", ""
		if spans.Length() >= 1 {
			category = strings.TrimSpace(spans.Eq(0).Text())
		}
		if spans.Length() >= 2 {
			version = strings.TrimSpace(spans.Eq(1).Text())
		}
		stat := link.Find("div.flex.items-center.justify-between div.flex.items-center").First()
		if stat.Length() > 0 {
			t := strings.TrimSpace(stat.Text())
			if i := strings.IndexByte(t, ' '); i > 0 {
				downloads = t[:i]
			} else {
				downloads = t
			}
		}

		icon, _ := link.Find("img").First().Attr("src")

		a := app{
			ID:        id,
			Title:     title,
			Category:  category,
			Version:   version,
			Downloads: downloads,
			Icon:      normalizeURL(icon),
			DetailURL: normalizeURL(href),
		}

		// 跟进详情页
		if detail && id != "" {
			time.Sleep(800 * time.Millisecond)
			ddoc, err := fetch("https://www.jiankuapp.com/app/detail?id="+id, timeout)
			if err != nil {
				log.Printf("详情页 %s 抓取失败: %v", id, err)
			} else {
				if raw, ok := ddoc.Find("div.markdown-body").First().Attr("data-raw"); ok {
					a.Description = cleanDesc(raw)
				}
				ddoc.Find("#download-modal a[href]").Each(func(_ int, s *goquery.Selection) {
					u, ok := s.Attr("href")
					if !ok || strings.TrimSpace(u) == "" {
						return
					}
					u = normalizeURL(u)
					if u == "" {
						return
					}
					name := strings.TrimSpace(s.Find(".font-medium").First().Text())
					if name == "" {
						name = hostFromURL(u)
					}
					a.Sources = append(a.Sources, item{Name: name, URL: u})
				})
			}
		}

		res.Apps = append(res.Apps, a)
	})

	return res
}

func buildURL(q string, page int) string {
	u := baseURL
	if strings.TrimSpace(q) != "" {
		u += "?q=" + url.QueryEscape(strings.TrimSpace(q))
		if page > 1 {
			u += fmt.Sprintf("&page=%d", page)
		}
	} else if page > 1 {
		u += fmt.Sprintf("?page=%d", page)
	}
	return u
}

func fetch(rawURL string, t time.Duration) (*goquery.Document, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; jkfetch/1.1)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: t}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func cleanDesc(raw string) string {
	raw = strings.ReplaceAll(raw, "\r", "")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "# ") {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, "\n")
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "unknown"
	}
	if parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	base := path.Base(parsed.Path)
	if base != "." && base != "/" {
		return base
	}
	return "unknown"
}

func normalizeURL(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "/") {
		return "https://www.jiankuapp.com" + raw
	}
	return ""
}

func fatal(format string, args ...interface{}) {
	log.Printf(format, args...)
	os.Exit(1)
}