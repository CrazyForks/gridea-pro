package domain

// Added json tags for frontend compatibility.

import (
	"context"
	"errors"
	"time"
)

// CommentPlatform 评论平台枚举
type CommentPlatform string

const (
	CommentPlatformValine CommentPlatform = "Valine"
	CommentPlatformWaline CommentPlatform = "Waline"
	CommentPlatformTwikoo CommentPlatform = "Twikoo"
	CommentPlatformGitalk CommentPlatform = "Gitalk"
	CommentPlatformGiscus CommentPlatform = "Giscus"
	CommentPlatformDisqus CommentPlatform = "Disqus"
	CommentPlatformCusdis CommentPlatform = "Cusdis"
)

// CommentSettings 评论设置 (Pure Entity)
type CommentSettings struct {
	Enable          bool                               `json:"enable"`
	Platform        CommentPlatform                    `json:"platform"`
	PlatformConfigs map[CommentPlatform]map[string]any `json:"platformConfigs"`
}

// Comment 统一评论模型 (Pure Entity)
type Comment struct {
	ID           string    `json:"id"`
	Avatar       string    `json:"avatar"`
	Nickname     string    `json:"nickname"`
	Email        string    `json:"email"`
	URL          string    `json:"url"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"createdAt" ts_type:"string"` // 使用 standard time.Time
	ArticleID    string    `json:"articleId"`
	ArticleTitle string    `json:"articleTitle"`
	ArticleURL   string    `json:"articleUrl"`
	ParentID     string    `json:"parentId"`
	ParentNick   string    `json:"parentNick"`
	IsNew        bool      `json:"isNew"`
}

// PaginatedComments 分页评论列表
type PaginatedComments struct {
	Comments   []Comment `json:"comments"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}

// Validate 校验评论数据
func (c *Comment) Validate() error {
	if c.Content == "" {
		return errors.New("comment content cannot be empty")
	}
	if c.Nickname == "" {
		return errors.New("comment nickname cannot be empty")
	}
	if c.ArticleID == "" {
		return errors.New("article ID is required")
	}
	return nil
}

// CommentRepository 评论存储接口
type CommentRepository interface {
	GetSettings(ctx context.Context) (*CommentSettings, error)
	SaveSettings(ctx context.Context, settings *CommentSettings) error
}

// CommentProvider 评论平台提供者接口
type CommentProvider interface {
	// GetComments 获取指定文章的评论
	GetComments(ctx context.Context, articleID string) ([]Comment, error)

	// GetRecentComments 获取最近评论
	GetRecentComments(ctx context.Context, limit int) ([]Comment, error)

	// GetAdminComments 获取管理端评论列表（支持分页）
	// 返回: comments, total, error
	GetAdminComments(ctx context.Context, page, pageSize int) ([]Comment, int64, error)

	// PostComment 发送评论/回复
	PostComment(ctx context.Context, comment *Comment) error

	// DeleteComment 删除评论
	DeleteComment(ctx context.Context, commentID string) error
}
