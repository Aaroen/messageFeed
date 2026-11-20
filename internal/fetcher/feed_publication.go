package fetcher

import (
	"context"
	"io"
	"messagefeed/internal/domain"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

func (c *Client) verifyPublicationDates(ctx context.Context, feedURL *url.URL, items []domain.Item) {
	if !initiumHost(feedURL.Hostname()) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(items)) {
		workers.Go(func() {
			for index := range jobs {
				if verified := c.articlePublicationTime(ctx, items[index].URL); verified != nil {
					items[index].PublishedAt = verified
					items[index].PublicationVerified = true
				}
			}
		})
	}
sendJobs:
	for index := range items {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	workers.Wait()
}

func initiumHost(host string) bool {
	return strings.EqualFold(host, "theinitium.com") || strings.EqualFold(host, "www.theinitium.com")
}

func (c *Client) articlePublicationTime(ctx context.Context, articleURL string) *time.Time {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil || request.URL.Scheme != "https" || !initiumHost(request.URL.Hostname()) {
		return nil
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "text/html")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !initiumHost(response.Request.URL.Hostname()) {
		return nil
	}
	return parseArticlePublicationTime(io.LimitReader(response.Body, 256*1024))
}

func parseArticlePublicationTime(body io.Reader) *time.Time {
	tokens := html.NewTokenizer(body)
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return nil
		case html.EndTagToken:
			name, _ := tokens.TagName()
			if string(name) == "head" {
				return nil
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokens.Token()
			if token.Data != "meta" {
				continue
			}
			var property, content string
			for _, attribute := range token.Attr {
				switch attribute.Key {
				case "property":
					property = attribute.Val
				case "content":
					content = attribute.Val
				}
			}
			if property == "article:published_time" {
				if parsed, err := time.Parse(time.RFC3339Nano, content); err == nil {
					value := parsed.UTC()
					return &value
				}
			}
		}
	}
}
