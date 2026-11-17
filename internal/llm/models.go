package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

func FetchModels(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("模型供应商地址无效")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("供应商模型目录读取失败，HTTP %d", response.StatusCode)
	}
	return decodeModelCatalog(response.Body)
}

func decodeModelCatalog(reader io.Reader) ([]Model, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (4<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("供应商模型目录超过大小限制")
	}
	var result struct {
		Data []Model `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Data == nil {
		return nil, fmt.Errorf("供应商模型目录缺少 data 数组")
	}
	seen := map[string]bool{}
	for _, model := range result.Data {
		if strings.TrimSpace(model.ID) != model.ID || model.ID == "" || len(model.ID) > 512 || seen[model.ID] {
			return nil, fmt.Errorf("供应商模型目录包含空白或重复标识")
		}
		seen[model.ID] = true
	}
	return result.Data, nil
}
