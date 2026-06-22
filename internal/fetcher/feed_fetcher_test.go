package fetcher

import (
	"context"
	"messagefeed/internal/domain"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
	"unicode/utf8"

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

	client := NewClient(WithNow(func() time.Time {
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

	client := NewClient(WithMaxBodySize(5))
	_, err := client.Fetch(context.Background(), domain.Source{
		ID:  1,
		URL: server.URL,
	})
	if err == nil {
		t.Fatal("Fetch returned nil error")
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
	}, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
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
