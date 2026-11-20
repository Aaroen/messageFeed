package fetcher

import (
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

func feedPublicationTime(feedURL *url.URL, feedItem *gofeed.Item) *time.Time {
	if feedItem.PublishedParsed == nil {
		return nil
	}
	publishedAt := feedItem.PublishedParsed.UTC()
	if feedURL != nil && initiumHost(feedURL.Hostname()) && strings.TrimSuffix(feedURL.Path, "/") == "/rss" && strings.HasSuffix(strings.TrimSpace(feedItem.Published), " GMT") {
		publishedAt = publishedAt.Add(8 * time.Hour)
	}
	return &publishedAt
}

func initiumHost(host string) bool {
	return strings.EqualFold(host, "theinitium.com") || strings.EqualFold(host, "www.theinitium.com")
}
