package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Post represents a blog post
type Post struct {
	// Metadata
	Title            string    `json:"title"`
	Date             time.Time `json:"date" ts_type:"string"`
	Tags             []string  `json:"tags"`
	TagIDs           []string  `json:"tagIds"`
	Categories       []string  `json:"categories"`
	Published        bool      `json:"published"`
	HideInList       bool      `json:"hideInList"`
	IsTop            bool      `json:"isTop"`
	Feature          string    `json:"feature"`
	FeatureImagePath string    `json:"featureImagePath"`
	FeatureImage     FileInfo  `json:"featureImage"`

	// Content
	Content        string `json:"content"`
	FileName       string `json:"fileName"`
	DeleteFileName string `json:"deleteFileName"` // 用于重命名/删除
	Abstract       string `json:"abstract"`
}

// Validate validates the post
func (p *Post) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(p.FileName) == "" {
		return errors.New("filename is required")
	}
	return nil
}

// PostRepository defines the interface for post persistence
type PostRepository interface {
	// Standard CRUD
	Create(ctx context.Context, post *Post) error
	Update(ctx context.Context, post *Post) error
	Delete(ctx context.Context, fileName string) error
	GetByFileName(ctx context.Context, fileName string) (*Post, error)

	// List returns paginated posts.
	// page is 1-based. size is items per page.
	// Returns posts, total count, and error.
	List(ctx context.Context, page, size int) ([]Post, int64, error)

	// Deprecated: Use List instead
	GetAll(ctx context.Context) ([]Post, error)

	// Reload forces a rescan of the post directory
	Reload(ctx context.Context) error
}
