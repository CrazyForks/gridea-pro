package comment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// WalineProvider Waline 评论提供者
type WalineProvider struct {
	*BaseProvider
	config *WalineConfig
}

// NewWalineProvider 创建 Waline Provider
func NewWalineProvider(config *WalineConfig, proxyURL string, logger *slog.Logger) *WalineProvider {
	return &WalineProvider{
		BaseProvider: NewBaseProvider(15*time.Second, proxyURL, logger),
		config:       config,
	}
}

// Waline Comment Response
type walineResponse struct {
	Errno  int             `json:"errno"`
	Errmsg interface{}     `json:"errmsg"`
	Data   json.RawMessage `json:"data"` // List or Object depending on context
}

// Waline List Response Data
type walineListData struct {
	Data  []walineComment `json:"data"`
	Count int             `json:"count"`
}

type walineComment struct {
	ObjectId  interface{} `json:"objectId"` // Can be string or number
	Nick      string      `json:"nick"`
	Comment   string      `json:"comment"`
	Mail      string      `json:"mail"`
	Link      string      `json:"link"`
	Pid       interface{} `json:"pid"` // Can be string or number
	Rid       interface{} `json:"rid"` // Root ID
	Url       string      `json:"url"`
	CreatedAt interface{} `json:"createdAt"` // Can be string or number (timestamp)
	// Waline specific
	Type   string `json:"type"`
	Status string `json:"status"`
}

// Helper to convert interface{} to string
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.0f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case map[string]interface{}:
		// Handle erroneous object return in errmsg or others
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// parseWalineData parses the data field which can be a list or an object
func parseWalineData(data json.RawMessage) ([]walineComment, int, error) {
	var listData walineListData
	if len(data) == 0 || string(data) == "null" {
		return []walineComment{}, 0, nil
	}

	// Try parsing as Object (paginated)
	if err := json.Unmarshal(data, &listData); err == nil {
		if listData.Data != nil {
			return listData.Data, listData.Count, nil
		}
	}

	// Try parsing as Array (direct list)
	var list []walineComment
	if err := json.Unmarshal(data, &list); err == nil {
		return list, len(list), nil
	}

	return nil, 0, fmt.Errorf("failed to parse waline data: %s", string(data))
}

// parseWalineTime converts various time formats to time.Time
func parseWalineTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch val := v.(type) {
	case string:
		// Try parsing common formats
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", val); err == nil {
			return t
		}
		return time.Time{}
	case float64:
		// Milliseconds? Seconds? Usually ms in JS
		return time.UnixMilli(int64(val))
	case int64:
		return time.UnixMilli(val)
	default:
		return time.Time{}
	}
}

// GetAdminComments implementation
func (p *WalineProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	serverURL := strings.TrimSuffix(p.config.ServerURLs, "/")
	apiURL := fmt.Sprintf("%s/api/comment?type=list&page=%d&pageSize=%d", serverURL, page, pageSize)

	var result walineResponse
	// Use auth request logic
	err := p.executeAuthRequest(ctx, "GET", apiURL, nil, &result)
	if err != nil {
		if strings.Contains(err.Error(), "401") {
			// Fallback logic for 401
			p.logger.WarnContext(ctx, "Waline Admin Auth failed (401), falling back to public recent comments")
			recentComments, err := p.GetRecentComments(ctx, pageSize)
			if err != nil {
				return nil, 0, fmt.Errorf("%w: failed to fetch public comments after auth fail: %v", ErrAuthFailed, err)
			}
			return recentComments, int64(len(recentComments)), nil
		}
		return nil, 0, err
	}

	if result.Errno != 0 {
		return nil, 0, fmt.Errorf("%w: %s", ErrProviderError, toString(result.Errmsg))
	}

	commentsList, count, err := parseWalineData(result.Data)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrProviderError, err)
	}

	var comments []domain.Comment
	for _, c := range commentsList {
		comments = append(comments, p.convertComment(c))
	}

	return comments, int64(count), nil
}

func (p *WalineProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	serverURL := strings.TrimSuffix(p.config.ServerURLs, "/")
	apiURL := fmt.Sprintf("%s/api/comment?path=%s", serverURL, url.QueryEscape(articleID))

	p.logger.DebugContext(ctx, "Waline GetComments", "url", apiURL)

	var result walineResponse
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &result, nil); err != nil {
		return nil, err
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("%w: %s", ErrProviderError, toString(result.Errmsg))
	}

	commentsList, _, err := parseWalineData(result.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderError, err)
	}

	var comments []domain.Comment
	for _, c := range commentsList {
		comments = append(comments, p.convertComment(c))
	}
	return comments, nil
}

func (p *WalineProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	serverURL := strings.TrimSuffix(p.config.ServerURLs, "/")
	apiURL := fmt.Sprintf("%s/api/comment?type=recent&count=%d", serverURL, limit)

	var result walineResponse
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &result, nil); err != nil {
		return nil, err
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("%w: %s", ErrProviderError, toString(result.Errmsg))
	}

	commentsList, _, err := parseWalineData(result.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderError, err)
	}

	var comments []domain.Comment
	for _, c := range commentsList {
		comments = append(comments, p.convertComment(c))
	}
	return comments, nil
}

func (p *WalineProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	payload := map[string]interface{}{
		"nick":    comment.Nickname,
		"comment": comment.Content,
		"url":     comment.ArticleID,
		"ua":      "Gridea Pro",
	}

	if comment.Email != "" {
		payload["mail"] = comment.Email
	}
	if comment.URL != "" {
		payload["link"] = comment.URL
	}

	if comment.ParentID != "" {
		payload["pid"] = comment.ParentID
	}

	serverURL := strings.TrimSuffix(p.config.ServerURLs, "/")
	apiURL := fmt.Sprintf("%s/api/comment", serverURL)

	// If MasterKey is set, use auth request for potentially better permissions
	if p.config.MasterKey != "" {
		var result walineResponse
		if err := p.executeAuthRequest(ctx, "POST", apiURL, payload, &result); err != nil {
			return err
		}
		if result.Errno != 0 {
			return fmt.Errorf("%w: %s", ErrProviderError, toString(result.Errmsg))
		}
		return nil
	}

	// Normal anonymous post
	var result walineResponse
	if err := p.DoJSON(ctx, "POST", apiURL, payload, &result, nil); err != nil {
		return err
	}
	if result.Errno != 0 {
		return fmt.Errorf("%w: %s", ErrProviderError, toString(result.Errmsg))
	}

	return nil
}

func (p *WalineProvider) DeleteComment(ctx context.Context, commentID string) error {
	if p.config.MasterKey == "" {
		return fmt.Errorf("%w: Waline delete requires MasterKey", ErrAuthFailed)
	}

	serverURL := strings.TrimSuffix(p.config.ServerURLs, "/")
	apiURL := fmt.Sprintf("%s/api/comment/%s", serverURL, commentID)

	var result walineResponse
	err := p.executeAuthRequest(ctx, "DELETE", apiURL, nil, &result)
	if err != nil {
		if strings.Contains(err.Error(), "401") {
			return fmt.Errorf("%w: Master Key invalid or permission denied", ErrAuthFailed)
		}
		return err
	}

	if result.Errno != 0 {
		return fmt.Errorf("%w: %s", ErrProviderError, toString(result.Errmsg))
	}

	return nil
}

// Convert internal DTO to Domain Model
func (p *WalineProvider) convertComment(c walineComment) domain.Comment {
	// Fix protocol-relative URLs in content
	content := c.Comment
	content = strings.ReplaceAll(content, "src=\"//", "src=\"https://")
	content = strings.ReplaceAll(content, "src='//", "src='https://")

	return domain.Comment{
		ID:        toString(c.ObjectId),
		Nickname:  c.Nick,
		URL:       c.Link,
		Content:   content,
		CreatedAt: parseWalineTime(c.CreatedAt),
		ArticleID: c.Url,
		ParentID:  toString(c.Pid),
		Email:     c.Mail,
		Avatar:    p.getGravatar(c.Mail),
	}
}

// executeAuthRequest calls the API with multiple Authorization header formats until one succeeds
func (p *WalineProvider) executeAuthRequest(ctx context.Context, method, apiURL string, reqBody interface{}, respDest interface{}) error {
	key := strings.TrimSpace(p.config.MasterKey)
	if key == "" {
		return fmt.Errorf("master key is empty")
	}

	// Authorization candidates
	candidates := []string{}
	if strings.HasPrefix(key, "Bearer ") {
		candidates = append(candidates, key)
	} else {
		candidates = append(candidates, "Bearer "+key)
		candidates = append(candidates, key)
		candidates = append(candidates, "Token "+key)
	}

	var lastErr error
	for _, token := range candidates {
		headers := map[string]string{
			"Authorization": token,
		}

		p.logger.DebugContext(ctx, "Trying Waline Auth", "token_prefix", token[:min(len(token), 10)]+"...")

		err := p.DoJSON(ctx, method, apiURL, reqBody, respDest, headers)
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "status 401") || strings.Contains(err.Error(), "status 403") {
			lastErr = err
			continue
		}

		// Other errors (network, 500, etc.) return immediately
		return err
	}

	return fmt.Errorf("%w: all auth attempts failed: %v", ErrAuthFailed, lastErr)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *WalineProvider) getGravatar(email string) string {
	if email == "" {
		return ""
	}
	email = strings.TrimSpace(strings.ToLower(email))
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("https://cravatar.cn/avatar/%s?d=mp&v=1.4.14", hex.EncodeToString(hash[:]))
}
