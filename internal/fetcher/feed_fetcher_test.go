package fetcher

import (
	"context"
	"messagefeed/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
