package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/domain"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

const defaultEmbeddingModel = "text-embedding-3-small"
const embeddingDefaultOperation = "batch_embed"

type EmbeddingRequest struct {
	Input     []string
	Operation string
}

type EmbeddingResponse struct {
	Provider   string
	Model      string
	Embeddings [][]float32
}

type EmbeddingClient interface {
	Embed(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error)
}

type OpenAICompatibleEmbeddingConfig struct {
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	Timeout     time.Duration
	MaxAttempts int
	MinInterval time.Duration
	HTTPClient  *http.Client
}

type OpenAICompatibleEmbeddingClient struct {
	provider      string
	baseURL       string
	apiKey        string
	model         string
	httpClient    *http.Client
	maxAttempts   int
	minInterval   time.Duration
	mu            sync.Mutex
	lastRequestAt time.Time
}

type embeddingRequestBody struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponseBody struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewOpenAICompatibleEmbeddingClient(config OpenAICompatibleEmbeddingConfig) (*OpenAICompatibleEmbeddingClient, error) {
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		provider = "openai_compatible"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if parsed, err := url.Parse(baseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("scheme and host are required")
		}
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "embedding_invalid_base_url", "invalid embedding base url", "llm.openai_embedding.new", false, err)
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "embedding_missing_api_key", "embedding api key is required", "llm.openai_embedding.new", false, nil)
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = defaultEmbeddingModel
	}
	client := config.HTTPClient
	if client == nil {
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)}
	}
	maxAttempts := config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &OpenAICompatibleEmbeddingClient{
		provider:    provider,
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		httpClient:  client,
		maxAttempts: maxAttempts,
		minInterval: config.MinInterval,
	}, nil
}

func (c *OpenAICompatibleEmbeddingClient) Embed(ctx context.Context, request EmbeddingRequest) (response EmbeddingResponse, err error) {
	if c == nil {
		return EmbeddingResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "embedding_client_nil", "embedding client is nil", "llm.openai_embedding.embed", false, nil)
	}
	operation := strings.TrimSpace(request.Operation)
	if operation == "" {
		operation = embeddingDefaultOperation
	}
	input := make([]string, 0, len(request.Input))
	inputChars := 0
	for _, text := range request.Input {
		text = strings.TrimSpace(text)
		if text != "" {
			input = append(input, text)
			inputChars += len([]rune(text))
		}
	}
	if len(input) == 0 {
		return EmbeddingResponse{Provider: c.provider, Model: c.model}, nil
	}
	startedAt := time.Now()
	ctx, span := observability.StartSpan(ctx, "llm.openai_embedding.embed",
		attribute.String("llm.provider", c.provider),
		attribute.String("llm.model", c.model),
		attribute.String("embedding.operation", operation),
		attribute.Int("embedding.input_count", len(input)),
		attribute.Int("embedding.input_chars", inputChars),
	)
	defer func() {
		status := "succeeded"
		if err != nil {
			status = "failed"
		}
		dimension := 0
		if len(response.Embeddings) > 0 {
			dimension = len(response.Embeddings[0])
		}
		span.SetAttributes(
			attribute.String("embedding.status", status),
			attribute.Int("embedding.dimension", dimension),
			attribute.Int("embedding.output_count", len(response.Embeddings)),
		)
		metrics.AgentEmbeddingRequestsTotal.WithLabelValues(c.provider, c.model, operation, status).Inc()
		metrics.AgentEmbeddingDuration.WithLabelValues(c.provider, c.model, operation, status).Observe(time.Since(startedAt).Seconds())
		metrics.AgentEmbeddingBatchSize.WithLabelValues(c.provider, c.model, operation).Observe(float64(len(input)))
		metrics.AgentEmbeddingInputChars.WithLabelValues(c.provider, c.model, operation).Observe(float64(inputChars))
		observability.EndSpan(span, err)
	}()
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		response, err = c.embedOnce(ctx, input, operation)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if !embeddingErrorRetryable(err) || attempt == c.maxAttempts {
			break
		}
		delay := time.Duration(attempt) * 500 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return EmbeddingResponse{}, ctx.Err()
		case <-timer.C:
		}
	}
	return EmbeddingResponse{}, lastErr
}

func (c *OpenAICompatibleEmbeddingClient) embedOnce(ctx context.Context, input []string, operation string) (EmbeddingResponse, error) {
	if err := c.waitRateLimit(ctx); err != nil {
		return EmbeddingResponse{}, err
	}
	payload, err := json.Marshal(embeddingRequestBody{Model: c.model, Input: input})
	if err != nil {
		return EmbeddingResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return EmbeddingResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpStartedAt := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		recordEmbeddingExternalHTTPRequest(operation, c.baseURL, "error", time.Since(httpStartedAt))
		return EmbeddingResponse{}, err
	}
	defer httpResp.Body.Close()
	recordEmbeddingExternalHTTPRequest(operation, c.baseURL, strconv.Itoa(httpResp.StatusCode), time.Since(httpStartedAt))
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 8<<20))
	if err != nil {
		return EmbeddingResponse{}, err
	}
	var decoded embeddingResponseBody
	if err := json.Unmarshal(body, &decoded); err != nil {
		return EmbeddingResponse{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 || decoded.Error != nil {
		message := strings.TrimSpace(httpResp.Status)
		if decoded.Error != nil && strings.TrimSpace(decoded.Error.Message) != "" {
			message = decoded.Error.Message
		}
		return EmbeddingResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "embedding_request_failed", message, "llm.openai_embedding.embed", true, nil)
	}
	if len(decoded.Data) != len(input) {
		return EmbeddingResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "embedding_response_count_mismatch", "embedding response count does not match input count", "llm.openai_embedding.embed", true, nil)
	}
	sort.SliceStable(decoded.Data, func(i, j int) bool {
		return decoded.Data[i].Index < decoded.Data[j].Index
	})
	output := make([][]float32, 0, len(decoded.Data))
	dimension := 0
	for _, item := range decoded.Data {
		if len(item.Embedding) == 0 {
			return EmbeddingResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "embedding_empty_vector", "embedding response contains empty vector", "llm.openai_embedding.embed", false, nil)
		}
		if dimension == 0 {
			dimension = len(item.Embedding)
		}
		if len(item.Embedding) != dimension {
			return EmbeddingResponse{}, domain.NewAppError(domain.ErrorKindInvalidInput, "embedding_dimension_mismatch", "embedding response dimensions are inconsistent", "llm.openai_embedding.embed", false, nil)
		}
		output = append(output, append([]float32(nil), item.Embedding...))
	}
	model := strings.TrimSpace(decoded.Model)
	if model == "" {
		model = c.model
	}
	return EmbeddingResponse{
		Provider:   c.provider,
		Model:      model,
		Embeddings: output,
	}, nil
}

func (c *OpenAICompatibleEmbeddingClient) waitRateLimit(ctx context.Context) error {
	if c.minInterval <= 0 {
		return nil
	}
	c.mu.Lock()
	wait := c.minInterval - time.Since(c.lastRequestAt)
	if wait <= 0 {
		c.lastRequestAt = time.Now()
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	timer := time.NewTimer(wait)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
	}
	c.mu.Lock()
	c.lastRequestAt = time.Now()
	c.mu.Unlock()
	return nil
}

func embeddingErrorRetryable(err error) bool {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		return false
	}
	return appErr.Retryable
}

func recordEmbeddingExternalHTTPRequest(operation string, baseURL string, status string, duration time.Duration) {
	if status == "" {
		status = "unknown"
	}
	metrics.ExternalHTTPRequestsTotal.WithLabelValues("embedding_"+operation, llmHTTPHost(baseURL), status).Inc()
	metrics.ExternalHTTPRequestDuration.WithLabelValues("embedding_"+operation, llmHTTPHost(baseURL)).Observe(duration.Seconds())
}
