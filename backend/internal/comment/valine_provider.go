package comment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// ValineProvider LeanCloud/Valine 评论提供者
type ValineProvider struct {
	*BaseProvider
	config *ValineConfig
}

// NewValineProvider 创建 Valine Provider
func NewValineProvider(config *ValineConfig, logger *slog.Logger) *ValineProvider {
	if config.ServerURLs == "" {
		// 默认 LeanCloud API 域名 (主要用于国际版，国内版通常需要自定义域名)
		config.ServerURLs = "https://leancloud.cn"
	}
	return &ValineProvider{
		BaseProvider: NewBaseProvider(15*time.Second, logger),
		config:       config,
	}
}

// LeanCloud Comment 结构
type leanCloudComment struct {
	ObjectId  string `json:"objectId"`
	Nick      string `json:"nick"`
	Comment   string `json:"comment"`
	Mail      string `json:"mail"`
	Link      string `json:"link"`
	Pid       string `json:"pid"`
	Rid       string `json:"rid"`
	Pnick     string `json:"pnick"`
	Url       string `json:"url"` // Article URL path
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// LeanCloud Response 结构
type leanCloudResponse struct {
	Results []leanCloudComment `json:"results"`
}

func (p *ValineProvider) getHeaders() map[string]string {
	headers := map[string]string{
		"X-LC-Id": p.config.AppID,
	}
	if p.config.MasterKey != "" {
		headers["X-LC-Key"] = fmt.Sprintf("%s,master", p.config.MasterKey)
	} else {
		headers["X-LC-Key"] = p.config.AppKey
	}
	return headers
}

func (p *ValineProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	// Valine 使用 url 作为文章标识
	//构建查询条件: {"url": articleID}
	where := fmt.Sprintf(`{"url":"%s"}`, articleID)
	params := url.Values{}
	params.Add("where", where)
	params.Add("order", "-createdAt")

	apiURL := fmt.Sprintf("%s/1.1/classes/Comment?%s", p.config.ServerURLs, params.Encode())

	var result leanCloudResponse
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &result, p.getHeaders()); err != nil {
		return nil, err
	}

	comments := make([]domain.Comment, 0, len(result.Results))
	for _, c := range result.Results {
		comments = append(comments, p.convertComment(c))
	}

	return comments, nil
}

func (p *ValineProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("order", "-createdAt")

	apiURL := fmt.Sprintf("%s/1.1/classes/Comment?%s", p.config.ServerURLs, params.Encode())

	var result leanCloudResponse
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &result, p.getHeaders()); err != nil {
		return nil, err
	}

	comments := make([]domain.Comment, 0, len(result.Results))
	for _, c := range result.Results {
		comments = append(comments, p.convertComment(c))
	}

	return comments, nil
}

func (p *ValineProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	// Valine (LeanCloud) doesn't have a direct "Get All Comments" for admin easily without querying all classes.
	// But usually we just list 'Comment' class.
	// We need to implement pagination.

	if page < 1 {
		page = 1
	}
	skip := (page - 1) * pageSize
	limit := pageSize

	// 1. Get List
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment?skip=%d&limit=%d&order=-createdAt", strings.TrimRight(p.config.ServerURLs, "/"), skip, limit)

	var result leanCloudResponse
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &result, p.getHeaders()); err != nil {
		return nil, 0, err
	}

	comments := make([]domain.Comment, 0, len(result.Results))
	for _, c := range result.Results {
		comments = append(comments, p.convertComment(c))
	}

	// 2. Get Count
	countURL := fmt.Sprintf("%s/1.1/classes/Comment?count=1&limit=0", strings.TrimRight(p.config.ServerURLs, "/"))
	var countResult struct {
		Count int64 `json:"count"`
	}

	// Try to get count, but don't fail hard
	if err := p.DoJSON(ctx, "GET", countURL, nil, &countResult, p.getHeaders()); err != nil {
		p.logger.WarnContext(ctx, "Failed to fetch comment count", "err", err)
		return comments, int64(len(comments)), nil // Fallback count
	}

	return comments, countResult.Count, nil
}

func (p *ValineProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	// Valine Create Comment
	// POST /1.1/classes/Comment
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment", strings.TrimRight(p.config.ServerURLs, "/"))

	lcComment := map[string]interface{}{
		"nick":    comment.Nickname,
		"comment": comment.Content,
		"mail":    comment.Email,
		"link":    comment.URL,
		"url":     comment.ArticleID,
	}

	if comment.ParentID != "" {
		// Fetch parent to get ID/RID
		parent, err := p.getCommentByID(ctx, comment.ParentID)
		if err != nil {
			return fmt.Errorf("failed to fetch parent comment: %w", err)
		}
		lcComment["pid"] = parent.ObjectId
		if parent.Rid != "" {
			lcComment["rid"] = parent.Rid
		} else {
			lcComment["rid"] = parent.ObjectId
		}
		lcComment["pnick"] = parent.Nick
	}

	lcComment["ua"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.2 Safari/605.1.15"

	// Valine API returns 201 Created on success
	var result map[string]interface{} // Response usually contains objectId and createdAt
	if err := p.DoJSON(ctx, "POST", apiURL, lcComment, &result, p.getHeaders()); err != nil {
		return err
	}

	return nil
}

func (p *ValineProvider) DeleteComment(ctx context.Context, commentID string) error {
	if p.config.MasterKey == "" {
		return fmt.Errorf("%w: master key is required to delete comments", ErrAuthFailed)
	}

	// LeanCloud DELETE /1.1/classes/Comment/:objectId
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment/%s", strings.TrimRight(p.config.ServerURLs, "/"), commentID)

	// DoJSON also supports DELETE and checks for >= 400 errors
	if err := p.DoJSON(ctx, "DELETE", apiURL, nil, nil, p.getHeaders()); err != nil {
		return err
	}

	return nil
}

func (p *ValineProvider) getCommentByID(ctx context.Context, id string) (*leanCloudComment, error) {
	apiURL := fmt.Sprintf("%s/1.1/classes/Comment/%s", strings.TrimRight(p.config.ServerURLs, "/"), id)

	var comment leanCloudComment
	if err := p.DoJSON(ctx, "GET", apiURL, nil, &comment, p.getHeaders()); err != nil {
		return nil, err
	}
	return &comment, nil
}

func (p *ValineProvider) convertComment(c leanCloudComment) domain.Comment {
	return domain.Comment{
		ID:         c.ObjectId,
		Nickname:   c.Nick,
		URL:        c.Link,
		Content:    c.Comment,
		CreatedAt:  parseValineTime(c.CreatedAt),
		ArticleID:  c.Url,
		ParentID:   c.Pid,
		ParentNick: c.Pnick,
		Email:      c.Mail,
		Avatar:     p.getGravatar(c.Mail),
	}
}

func parseValineTime(t string) time.Time {
	// Try standard RFC3339 first
	parsed, err := time.Parse(time.RFC3339, t)
	if err == nil {
		return parsed
	}
	// Try other formats if needed, or return current time/zero time
	return time.Now()
}

func (p *ValineProvider) getGravatar(email string) string {
	if email == "" {
		return ""
	}
	email = strings.TrimSpace(strings.ToLower(email))
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("https://cravatar.cn/avatar/%s?d=mp&v=1.4.14", hex.EncodeToString(hash[:]))
}
