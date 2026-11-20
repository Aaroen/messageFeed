package fetcher

import (
	"context"
	"messagefeed/internal/domain"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

func TestFetchParsesRSSItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example Feed</title>
    <link>https://example.com</link>
    <item>
      <title>First Item</title>
      <link>/posts/first#section</link>
      <guid>first-guid</guid>
      <description>First description</description>
      <pubDate>Tue, 16 Jun 2026 09:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer server.Close()

	client := NewClient(WithHTTPClient(server.Client()), WithNow(func() time.Time {
		return time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	}))

	result, err := client.Fetch(context.Background(), domain.Source{
		ID:  7,
		URL: server.URL + "/feed.xml",
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if got, want := len(result.Items), 1; got != want {
		t.Fatalf("items length = %d, want %d", got, want)
	}
	item := result.Items[0]
	if item.SourceID != 7 {
		t.Fatalf("SourceID = %d, want 7", item.SourceID)
	}
	if item.Title != "First Item" {
		t.Fatalf("Title = %q", item.Title)
	}
	if item.NormalizedURL != server.URL+"/posts/first" {
		t.Fatalf("NormalizedURL = %q", item.NormalizedURL)
	}
	if item.RawGUID != "first-guid" {
		t.Fatalf("RawGUID = %q", item.RawGUID)
	}
	if item.PublishedAt == nil {
		t.Fatal("PublishedAt is nil")
	}
	if item.ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
}

func TestFetchRejectsLargeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("123456"))
	}))
	defer server.Close()

	client := NewClient(WithHTTPClient(server.Client()), WithMaxBodySize(5))
	_, err := client.Fetch(context.Background(), domain.Source{
		ID:  1,
		URL: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "response exceeds 5 bytes") {
		t.Fatalf("Fetch error = %v", err)
	}
}

func TestFeedOnlyUsesPublicAddresses(test *testing.T) {
	for address, allowed := range map[string]bool{
		"8.8.8.8": true, "2001:4860:4860::8888": true,
		"127.0.0.1": false, "::ffff:127.0.0.1": false, "10.1.1.1": false,
		"100.64.0.1": false, "169.254.169.254": false, "fd00::1": false,
	} {
		if actual := publicFeedAddress(netip.MustParseAddr(address)); actual != allowed {
			test.Errorf("publicFeedAddress(%s) = %v, want %v", address, actual, allowed)
		}
	}
}

func TestFeedPublicationTime(test *testing.T) {
	fetchedAt := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	for _, scenario := range []struct {
		name      string
		feedURL   string
		pubDate   string
		published string
		verified  bool
	}{
		{"initium shifted", "https://theinitium.com/rss", "Thu, 19 Nov 2026 23:00:00 GMT", "2026-11-20T07:00:00Z", false},
		{"initium already UTC", "https://theinitium.com/rss/", "Fri, 20 Nov 2026 07:15:00 GMT", "2026-11-20T07:15:00Z", true},
		{"initium explicit zone", "https://theinitium.com/rss/", "2026-11-20T07:00:00Z", "2026-11-20T07:00:00Z", false},
		{"other initium feed", "https://theinitium.com/another/feed", "Thu, 19 Nov 2026 23:00:00 GMT", "2026-11-19T23:00:00Z", false},
		{"infoq local time", "https://www.infoq.cn/feed", "Fri, 20 Nov 2026 14:00:00 GMT", "2026-11-20T06:00:00Z", true},
		{"infoq explicit offset", "https://www.infoq.cn/feed", "Fri, 20 Nov 2026 14:00:00 +0800", "2026-11-20T06:00:00Z", false},
		{"other feed", "https://example.com/rss", "Thu, 19 Nov 2026 23:00:00 GMT", "2026-11-19T23:00:00Z", false},
		{"future date", "https://example.com/rss", "Fri, 20 Nov 2026 09:00:00 GMT", "", false},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			feedURL, err := url.Parse(scenario.feedURL)
			if err != nil {
				test.Fatal(err)
			}
			feed, err := gofeed.NewParser().ParseString(`<rss version="2.0"><channel><item><title>Article</title><link>/article</link><pubDate>` + scenario.pubDate + `</pubDate></item></channel></rss>`)
			if err != nil {
				test.Fatal(err)
			}
			item, ok := feedItemToDomain(1, feedURL, feed.Items[0], nil, fetchedAt)
			actual := ""
			if item.PublishedAt != nil {
				actual = item.PublishedAt.Format(time.RFC3339)
			}
			if !ok || actual != scenario.published || item.PublicationVerified != scenario.verified {
				test.Fatalf("published = %v, want %s", item.PublishedAt, scenario.published)
			}
		})
	}
}

func TestXinhuaArchivePublicationDate(test *testing.T) {
	const link = "http://www.news.cn/politics/2022-12/08/c_1129192044.htm"
	dates, err := xinhuaArchiveDates([]byte(`<rss version="2.0"><channel><item>
<title><![CDATA[关于印发新冠病毒抗原检测应用方案的通知]]></title>
<link>` + link + `</link>Thu,08-Dec-2022 14:55:13 GMT
<description><![CDATA[<a href=` + link + `>通知</a>]]></description></item></channel></rss>`))
	if err != nil || dates[link].Format(time.RFC3339) != "2022-12-08T06:55:13Z" {
		test.Fatalf("archive date: %v, %v", dates[link], err)
	}
}

func TestArticlePublicationIgnoresModifiedDate(test *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><head>
<meta name="article:modified_time" content="2026-07-09T12:42:32.677Z"/>
<script type="application/ld+json">{"@type":"ReportageNewsArticle","datePublished":"2026-07-08T00:00:59.557Z","dateModified":"2026-07-09T12:42:32.677Z"}</script>
</head></html>`))
	if err != nil {
		test.Fatal(err)
	}
	if actual := articlePublicationFromHTML(document).Format(time.RFC3339Nano); actual != "2026-07-08T00:00:59.557Z" {
		test.Fatalf("publication date = %s", actual)
	}
}

func TestTruncatePreservesUTF8(t *testing.T) {
	value := "消息流刷新测试，包含中文全角字符：，。"
	got := truncate(value, 40)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate returned invalid UTF-8: %q", got)
	}
	if len(got) > 40 {
		t.Fatalf("truncate length = %d, want <= 40", len(got))
	}
}

func TestFeedItemToDomainRemovesInvalidUTF8(t *testing.T) {
	baseURL, err := url.Parse("https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	invalidSuffix := string([]byte{0xe6, 0xb5})
	item, ok := feedItemToDomain(3, baseURL, &gofeed.Item{
		GUID:    "guid" + invalidSuffix,
		Link:    "/posts/item" + invalidSuffix,
		Title:   "Title" + invalidSuffix,
		Content: "Body " + invalidSuffix + " text",
		Author:  &gofeed.Person{Name: "Author" + invalidSuffix},
	}, nil, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("feedItemToDomain returned ok=false")
	}

	values := map[string]string{
		"title":           item.Title,
		"url":             item.URL,
		"normalized_url":  item.NormalizedURL,
		"raw_guid":        item.RawGUID,
		"content_snippet": item.ContentSnippet,
		"author":          item.Author,
	}
	for name, value := range values {
		if !utf8.ValidString(value) {
			t.Fatalf("%s contains invalid UTF-8: %q", name, value)
		}
	}
	if item.Title != "Title" {
		t.Fatalf("Title = %q, want Title", item.Title)
	}
	if item.RawGUID != "guid" {
		t.Fatalf("RawGUID = %q, want guid", item.RawGUID)
	}
	if item.NormalizedURL != "https://example.com/posts/item" {
		t.Fatalf("NormalizedURL = %q", item.NormalizedURL)
	}
}
