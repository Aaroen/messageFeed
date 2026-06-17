package fetcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultMaxBodyBytes = 10 * 1024 * 1024
	defaultUserAgent    = "messageFeed/0.1"
)

type Client struct {
	httpClient  *http.Client
	maxBodySize int64
	userAgent   string
	now         func() time.Time
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithMaxBodySize(maxBodySize int64) Option {
	return func(client *Client) {
		if maxBodySize > 0 {
			client.maxBodySize = maxBodySize
		}
	}
}

func WithNow(now func() time.Time) Option {
	return func(client *Client) {
		if now != nil {
			client.now = now
		}
	}
}

func NewClient(options ...Option) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		maxBodySize: defaultMaxBodyBytes,
		userAgent:   defaultUserAgent,
		now:         time.Now,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func (c *Client) Fetch(ctx context.Context, source domain.Source) (domain.FeedFetchResult, error) {
	ctx, span := observability.StartSpan(ctx, "fetcher.feed.fetch",
		attribute.Int64("source.id", source.ID),
		attribute.String("source.name", source.Name),
		attribute.String("feed.url", source.URL),
	)
	var fetchErr error
	defer func() {
		observability.EndSpan(span, fetchErr)
	}()

	if c == nil {
		c = NewClient()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		fetchErr = fmt.Errorf("build feed request: %w", err)
		return domain.FeedFetchResult{}, fetchErr
	}
	if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
		fetchErr = fmt.Errorf("%w: feed url scheme must be http or https", domain.ErrInvalidInput)
		return domain.FeedFetchResult{}, fetchErr
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/json, application/xml, text/xml, */*")

	httpStartedAt := time.Now()
	response, err := c.httpClient.Do(request)
	if err != nil {
		recordExternalHTTPRequest(request.URL.Host, "error", time.Since(httpStartedAt))
		fetchErr = fmt.Errorf("fetch feed: %w", err)
		return domain.FeedFetchResult{}, fetchErr
	}
	defer response.Body.Close()
	recordExternalHTTPRequest(request.URL.Host, strconv.Itoa(response.StatusCode), time.Since(httpStartedAt))
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		fetchErr = fmt.Errorf("fetch feed: unexpected status %d", response.StatusCode)
		return domain.FeedFetchResult{}, fetchErr
	}

	body, err := readLimited(response.Body, c.maxBodySize)
	if err != nil {
		fetchErr = err
		return domain.FeedFetchResult{}, fetchErr
	}
	span.SetAttributes(attribute.Int("feed.body_bytes", len(body)))

	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		fetchErr = fmt.Errorf("parse feed: %w", err)
		return domain.FeedFetchResult{}, fetchErr
	}

	baseURL, err := url.Parse(source.URL)
	if err != nil {
		fetchErr = fmt.Errorf("parse source url: %w", err)
		return domain.FeedFetchResult{}, fetchErr
	}

	fetchedAt := c.now().UTC()
	items := make([]domain.Item, 0, len(feed.Items))
	for _, feedItem := range feed.Items {
		item, ok := feedItemToDomain(source.ID, baseURL, feedItem, fetchedAt)
		if !ok {
			continue
		}
		items = append(items, item)
	}

	span.SetAttributes(attribute.Int("feed.items", len(items)))
	return domain.FeedFetchResult{Items: items}, nil
}

func recordExternalHTTPRequest(host string, status string, duration time.Duration) {
	if host == "" {
		host = "unknown"
	}
	metrics.ExternalHTTPRequestsTotal.WithLabelValues("feed_fetch", host, status).Inc()
	metrics.ExternalHTTPRequestDuration.WithLabelValues("feed_fetch", host).Observe(duration.Seconds())
}

func readLimited(reader io.Reader, maxBodySize int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBodySize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read feed body: %w", err)
	}
	if int64(len(body)) > maxBodySize {
		return nil, fmt.Errorf("read feed body: response exceeds %d bytes", maxBodySize)
	}
	return body, nil
}

func feedItemToDomain(sourceID int64, baseURL *url.URL, feedItem *gofeed.Item, fetchedAt time.Time) (domain.Item, bool) {
	if feedItem == nil {
		return domain.Item{}, false
	}

	rawGUID := strings.TrimSpace(feedItem.GUID)
	rawLink := strings.TrimSpace(feedItem.Link)
	if rawLink == "" && looksLikeAbsoluteURL(rawGUID) {
		rawLink = rawGUID
	}

	itemURL, normalizedURL, ok := resolveAndNormalizeURL(baseURL, rawLink)
	if !ok {
		return domain.Item{}, false
	}

	title := strings.TrimSpace(feedItem.Title)
	if title == "" {
		title = normalizedURL
	}

	content := strings.TrimSpace(feedItem.Content)
	if content == "" {
		content = strings.TrimSpace(feedItem.Description)
	}
	contentSnippet := truncate(normalizeWhitespace(content), 2000)

	var publishedAt *time.Time
	if feedItem.PublishedParsed != nil {
		value := feedItem.PublishedParsed.UTC()
		publishedAt = &value
	} else if feedItem.UpdatedParsed != nil {
		value := feedItem.UpdatedParsed.UTC()
		publishedAt = &value
	}

	return domain.Item{
		SourceID:       sourceID,
		Title:          title,
		URL:            itemURL,
		NormalizedURL:  normalizedURL,
		RawGUID:        rawGUID,
		ContentHash:    contentHash(title, normalizedURL, content),
		ContentSnippet: contentSnippet,
		Author:         authorName(feedItem),
		PublishedAt:    publishedAt,
		FetchedAt:      fetchedAt,
	}, true
}

func resolveAndNormalizeURL(baseURL *url.URL, rawLink string) (itemURL string, normalizedURL string, ok bool) {
	if strings.TrimSpace(rawLink) == "" {
		return "", "", false
	}

	parsed, err := url.Parse(strings.TrimSpace(rawLink))
	if err != nil {
		return "", "", false
	}
	if !parsed.IsAbs() && baseURL != nil {
		parsed = baseURL.ResolveReference(parsed)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", false
	}
	if parsed.Host == "" {
		return "", "", false
	}
	if parsed.Path == "/" {
		parsed.Path = ""
	}

	normalizedURL = parsed.String()
	return normalizedURL, normalizedURL, true
}

func looksLikeAbsoluteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.IsAbs() && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func authorName(item *gofeed.Item) string {
	if item.Author == nil {
		return ""
	}
	if name := strings.TrimSpace(item.Author.Name); name != "" {
		return name
	}
	return strings.TrimSpace(item.Author.Email)
}

func contentHash(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncate(value string, maxLength int) string {
	if maxLength < 1 || len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}
