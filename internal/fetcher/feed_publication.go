package fetcher

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"messagefeed/internal/domain"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

const xinhuaPoliticsArchivePath = "/politics/news_politics.xml"

func IsXinhuaPoliticsArchive(feedURL *url.URL) bool {
	return feedURL != nil && strings.EqualFold(feedURL.Hostname(), "www.xinhuanet.com") && feedURL.Path == xinhuaPoliticsArchivePath
}

func xinhuaArchiveDates(body []byte) (map[string]time.Time, error) {
	var archive struct {
		Items []struct {
			Link string `xml:"link"`
			Date string `xml:",chardata"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(body, &archive); err != nil {
		return nil, err
	}
	dates := make(map[string]time.Time, len(archive.Items))
	beijing := time.FixedZone("UTC+08:00", 8*60*60)
	for _, item := range archive.Items {
		date, err := time.ParseInLocation("Mon,02-Jan-2006 15:04:05", strings.TrimSuffix(strings.TrimSpace(item.Date), " GMT"), beijing)
		if err == nil {
			dates[strings.TrimSpace(item.Link)] = date.UTC()
		}
	}
	return dates, nil
}

func feedPublicationTime(feedURL *url.URL, feedItem *gofeed.Item, feedUpdatedAt *time.Time, fetchedAt time.Time) (*time.Time, bool) {
	if feedItem.PublishedParsed == nil {
		return nil, false
	}
	publishedAt := feedItem.PublishedParsed.UTC()
	verified := false
	if feedURL != nil && strings.HasSuffix(strings.TrimSpace(feedItem.Published), " GMT") {
		host := strings.ToLower(feedURL.Hostname())
		path := strings.TrimSuffix(feedURL.Path, "/")
		switch {
		case (host == "infoq.cn" || host == "www.infoq.cn") && path == "/feed":
			publishedAt = publishedAt.Add(-8 * time.Hour)
			verified = true
		case (host == "theinitium.com" || host == "www.theinitium.com") && path == "/rss":
			ceiling := fetchedAt
			if feedUpdatedAt != nil && feedUpdatedAt.Before(ceiling) && !feedUpdatedAt.Before(publishedAt) {
				ceiling = *feedUpdatedAt
			}
			shifted := publishedAt.Add(8 * time.Hour)
			articleURL, _ := url.Parse(feedItem.Link)
			knownConflict := articleURL != nil && strings.TrimSuffix(articleURL.Path, "/") == "/20261120-whatsnew-apec-shenzhen-explained"
			if !knownConflict && !shifted.After(ceiling) {
				publishedAt = shifted
			} else {
				verified = true
			}
		}
	}
	if publishedAt.After(fetchedAt) {
		return nil, false
	}
	return &publishedAt, verified
}

func publicationArticleFeed(feedURL *url.URL) bool {
	host := strings.ToLower(feedURL.Hostname())
	return host == "feeds.bbci.co.uk" || host == "feeds.bbc.co.uk" ||
		host == "www.bbc.com" && strings.Contains(feedURL.Path, "rss") ||
		host == "sspai.com" && strings.TrimSuffix(feedURL.Path, "/") == "/feed"
}

func (c *Client) verifyArticlePublicationDates(ctx context.Context, items []domain.Item) {
	deadline := time.Second
	if requestDeadline, ok := ctx.Deadline(); ok {
		deadline = min(deadline, time.Until(requestDeadline)/2)
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	var workers sync.WaitGroup
	for index := range items {
		articleURL, err := url.Parse(items[index].URL)
		if err != nil {
			continue
		}
		host := articleURL.Hostname()
		bbcArticle := (host == "www.bbc.com" || host == "www.bbc.co.uk") && strings.Contains(articleURL.Path, "/articles/")
		sspaiArticle := host == "sspai.com" && strings.HasPrefix(articleURL.Path, "/post/")
		if !bbcArticle && !sspaiArticle {
			continue
		}
		articleURL.RawQuery = ""
		articleURL.Scheme = "https"
		item := &items[index]
		if bbcArticle {
			item.PublishedAt = nil
		}
		workers.Go(func() {
			date := c.articlePublicationDate(ctx, articleURL.String())
			if !date.IsZero() && !date.After(item.FetchedAt) {
				item.PublishedAt = &date
				item.PublicationVerified = true
			}
		})
	}
	workers.Wait()
}

func (c *Client) articlePublicationDate(ctx context.Context, articleURL string) time.Time {
	c.articleMu.Lock()
	cached := c.articleDates[articleURL]
	c.articleMu.Unlock()
	if !cached.IsZero() {
		return cached
	}
	select {
	case c.articleSlots <- struct{}{}:
		defer func() { <-c.articleSlots }()
	case <-ctx.Done():
		return time.Time{}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return time.Time{}
	}
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return time.Time{}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return time.Time{}
	}
	document, err := goquery.NewDocumentFromReader(io.LimitReader(response.Body, 128*1024))
	if err != nil {
		return time.Time{}
	}
	date := articlePublicationFromHTML(document)
	if !date.IsZero() {
		c.articleMu.Lock()
		if len(c.articleDates) >= 512 {
			clear(c.articleDates)
		}
		c.articleDates[articleURL] = date
		c.articleMu.Unlock()
	}
	return date
}

func articlePublicationFromHTML(document *goquery.Document) time.Time {
	value, _ := document.Find(`head meta[property="article:published_time"], head meta[name="article:published_time"]`).First().Attr("content")
	date, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return date.UTC()
	}
	document.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, script *goquery.Selection) bool {
		var article struct {
			PublishedAt time.Time `json:"datePublished"`
		}
		if err := json.Unmarshal([]byte(script.Text()), &article); err == nil && !article.PublishedAt.IsZero() {
			date = article.PublishedAt
			return false
		}
		return true
	})
	return date.UTC()
}
