package comment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// BaseProvider 提供所有 CommentProvider 共用的基础功能
// 包含 HTTP Client 配置、日志记录、重试逻辑等
type BaseProvider struct {
	client *http.Client
	logger *slog.Logger
}

// NewBaseProvider 创建基础 Provider
// timeout: HTTP 请求超时时间，默认 30s
func NewBaseProvider(timeout time.Duration, logger *slog.Logger) *BaseProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &BaseProvider{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		logger: logger,
	}
}

// DoRequest 发送 HTTP 请求 (raw response)
func (b *BaseProvider) DoRequest(ctx context.Context, method, url string, body []byte, headers map[string]string) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 默认 Content-Type 为 JSON，如果 headers 中有覆盖则以 headers 为准
	if req.Header.Get("Content-Type") == "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		req.Header.Set("Content-Type", "application/json")
	}

	b.logger.DebugContext(ctx, "Sending HTTP request", "method", method, "url", url)

	resp, err := b.client.Do(req)
	if err != nil {
		// 检查是否是 Context 取消或超时
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// DoJSON 发送请求并将响应解析为 JSON
// reqBody: 请求体 (会被 marshal 为 JSON)，可为 nil
// respDest: 响应解析目标 (传指针)，可为 nil
func (b *BaseProvider) DoJSON(ctx context.Context, method, url string, reqBody interface{}, respDest interface{}, headers map[string]string) error {
	var bodyBytes []byte
	var err error

	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	resp, err := b.DoRequest(ctx, method, url, bodyBytes, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// 尝试读取 response body 以获取错误信息
		respBody, _ := io.ReadAll(resp.Body)
		b.logger.WarnContext(ctx, "API request returned error status",
			"status", resp.StatusCode,
			"url", url,
			"body", string(respBody),
		)

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: status %d", ErrAuthFailed, resp.StatusCode)
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: status %d", ErrNotFound, resp.StatusCode)
		}
		return fmt.Errorf("%w: status %d, body: %s", ErrProviderError, resp.StatusCode, string(respBody))
	}

	if respDest != nil {
		if err := json.NewDecoder(resp.Body).Decode(respDest); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
