package comment

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"log/slog"
	"time"
)

// TwikooProvider Twikoo 评论提供者
type TwikooProvider struct {
	*BaseProvider
	config *TwikooConfig
}

// NewTwikooProvider 创建 Twikoo Provider
func NewTwikooProvider(config *TwikooConfig, logger *slog.Logger) *TwikooProvider {
	return &TwikooProvider{
		BaseProvider: NewBaseProvider(15*time.Second, logger),
		config:       config,
	}
}

// Twikoo API Request
type twikooRequest struct {
	Event string `json:"event"`
	// get-recent-comments params
	IncludeReply bool `json:"includeReply,omitempty"`
	PageSize     int  `json:"pageSize,omitempty"`
	// comment-get params
	Url      string `json:"url,omitempty"`
	Admin    bool   `json:"admin,omitempty"`
	Page     int    `json:"page,omitempty"`
	ParentId string `json:"parentId,omitempty"`
	// comment-submit params
	Nick    string `json:"nick,omitempty"`
	Mail    string `json:"mail,omitempty"`
	Link    string `json:"link,omitempty"`
	Comment string `json:"comment,omitempty"`
	Pid     string `json:"pid,omitempty"`
	Rid     string `json:"rid,omitempty"`
	Ua      string `json:"ua,omitempty"`
}

// Twikoo Comment Data Structure
type twikooComment struct {
	ID       string          `json:"id"`
	Nick     string          `json:"nick"`
	Mail     string          `json:"mail"`
	Link     string          `json:"link"`
	Comment  string          `json:"comment"` // HTML content
	Url      string          `json:"url"`
	ParentId string          `json:"pid"`
	Rid      string          `json:"rid"`
	Created  int64           `json:"created"`
	Updated  int64           `json:"updated"`
	Avatar   string          `json:"avatar"`
	IsSpam   bool            `json:"isSpam"`
	Top      bool            `json:"top"`
	Role     string          `json:"role"`
	Replies  []twikooComment `json:"replies"`
}

type twikooResponse struct {
	Code int             `json:"code"`
	Data []twikooComment `json:"data"` // For get list
	Msg  string          `json:"msg"`
}

func (p *TwikooProvider) getAPIUrl() string {
	// Twikoo 云函数通常是 EnvID 本身如果是 URL，或者特定的云厂商格式
	// 简单起见，这里假设 EnvID 是完整的云函数 URL，或者用户需要填完整的 URL
	return p.config.EnvID
}

func (p *TwikooProvider) callAPI(ctx context.Context, payload twikooRequest) (*twikooResponse, error) {
	var result twikooResponse
	// Twikoo usually returns 200 even for logical errors, but check Code
	if err := p.DoJSON(ctx, "POST", p.getAPIUrl(), payload, &result, nil); err != nil {
		return nil, err
	}

	if result.Code != 0 && result.Code != 200 { // Twikoo 有时返回 0 或 200 表示成功
		return nil, fmt.Errorf("%w: %s", ErrProviderError, result.Msg)
	}

	return &result, nil
}

// GetAdminComments implementation
func (p *TwikooProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	// Twikoo Cloud Function API for getting comments logic needed here
	// For now fallback to recent
	// TODO: Implement proper admin list if Twikoo supports it via standard API
	comments, err := p.GetRecentComments(ctx, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// Fake count
	return comments, int64(len(comments)), nil
}

func (p *TwikooProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	payload := twikooRequest{
		Event: "comment-get",
		Url:   articleID,
		Admin: true, // 获取全部评论可能需要
	}

	resp, err := p.callAPI(ctx, payload)
	if err != nil {
		return nil, err
	}

	return p.convertComments(resp.Data), nil
}

func (p *TwikooProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	payload := twikooRequest{
		Event:        "get-recent-comments",
		IncludeReply: true,
		PageSize:     limit,
	}

	resp, err := p.callAPI(ctx, payload)
	if err != nil {
		return nil, err
	}

	return p.convertComments(resp.Data), nil
}

func (p *TwikooProvider) convertComments(tComments []twikooComment) []domain.Comment {
	var comments []domain.Comment
	for _, c := range tComments {
		// Twikoo's 'created' is a Unix timestamp in milliseconds
		createdTime := time.UnixMilli(c.Created)

		comments = append(comments, domain.Comment{
			ID:        c.ID,
			Avatar:    c.Avatar,
			Nickname:  c.Nick,
			URL:       c.Link,
			Content:   c.Comment,
			CreatedAt: createdTime,
			ArticleID: c.Url,
			ParentID:  c.ParentId,
		})

		// 处理嵌套回复
		if len(c.Replies) > 0 {
			comments = append(comments, p.convertComments(c.Replies)...)
		}
	}
	return comments
}

func (p *TwikooProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	payload := twikooRequest{
		Event:   "comment-submit",
		Nick:    comment.Nickname,
		Mail:    "", // 可选
		Link:    comment.URL,
		Comment: comment.Content,
		Url:     comment.ArticleID,
		Pid:     comment.ParentID,
		Rid:     comment.ParentID, // Root ID 简化处理
		Ua:      "Gridea Pro Desktop",
	}

	_, err := p.callAPI(ctx, payload)
	return err
}

func (p *TwikooProvider) DeleteComment(ctx context.Context, commentID string) error {
	// Twikoo 删除通常需要 accessToken，目前 API 暂不支持直接通过公共接口删除
	// 可能需要实现管理端专用接口
	return fmt.Errorf("%w: Twikoo delete not supported yet", ErrNotImplemented)
}
